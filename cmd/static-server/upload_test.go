package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestHandler(t *testing.T) (*handler, string) {
	t.Helper()
	root := t.TempDir()
	cfg := serverConfig{
		signSecret:      "test-secret",
		apiToken:        "test-token",
		mediaRoot:       root,
		publicBaseURL:   "https://static.example.com",
		allowedPrefixes: map[string]struct{}{"orderflow": {}},
		maxUploadBytes:  1024 * 1024,
	}
	return newHandler(cfg), root
}

// buildMultipart는 테스트용 multipart 요청 body를 만든다.
func buildMultipart(t *testing.T, fields map[string]string, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
		hdr.Set("Content-Type", "application/octet-stream")
		part, err := mw.CreatePart(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return body, mw.FormDataContentType()
}

func TestUploadSuccess(t *testing.T) {
	h, root := newTestHandler(t)

	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
		"prefix":     "orders/123",
	}, "invoice.pdf", []byte("PDF-BYTES"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Path, "/private/orderflow/orders/123/") {
		t.Errorf("unexpected path: %s", resp.Path)
	}
	if !strings.HasSuffix(resp.Path, ".pdf") {
		t.Errorf("path should keep extension: %s", resp.Path)
	}
	if resp.Size != int64(len("PDF-BYTES")) {
		t.Errorf("unexpected size: %d", resp.Size)
	}

	// 실제 파일이 디스크에 있는지 확인
	full := filepath.Join(root, resp.Path)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("stored file not readable: %v", err)
	}
	if string(data) != "PDF-BYTES" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

func TestUploadPublicVisibility(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "public",
		"app":        "orderflow",
		"prefix":     "products",
	}, "menu.jpg", []byte("jpgdata"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp uploadResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.HasPrefix(resp.Path, "/public/orderflow/products/") {
		t.Errorf("unexpected path: %s", resp.Path)
	}
}

func TestUploadNoBearer(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
	}, "x.txt", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	// no Authorization
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestUploadWrongBearer(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
	}, "x.txt", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestUploadBadVisibility(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "weird",
		"app":        "orderflow",
	}, "x.txt", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BAD_VISIBILITY") {
		t.Errorf("expected BAD_VISIBILITY: %s", rec.Body.String())
	}
}

func TestUploadUnknownApp(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "nope",
	}, "x.txt", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BAD_APP") {
		t.Errorf("expected BAD_APP: %s", rec.Body.String())
	}
}

func TestUploadTooLarge(t *testing.T) {
	h, _ := newTestHandler(t)
	h.cfg.maxUploadBytes = 10

	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
	}, "x.txt", bytes.Repeat([]byte("A"), 1024))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUploadPrefixTraversal(t *testing.T) {
	h, _ := newTestHandler(t)
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
		"prefix":     "../evil",
	}, "x.txt", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BAD_PREFIX") {
		t.Errorf("expected BAD_PREFIX: %s", rec.Body.String())
	}
}

func TestUploadExtensionSanitized(t *testing.T) {
	h, _ := newTestHandler(t)
	// 확장자에 이상한 문자 → 확장자 제거
	body, ct := buildMultipart(t, map[string]string{
		"visibility": "private",
		"app":        "orderflow",
	}, "malicious.ph p", []byte("x"))

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp uploadResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if strings.Contains(resp.Path, " ") {
		t.Errorf("unexpected space in path: %s", resp.Path)
	}
}

