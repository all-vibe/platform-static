package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/all-vibe/platform-static/pkg/signer"
)

func TestHealthz(t *testing.T) {
	h := newHandler(serverConfig{
		signSecret:      "test-secret",
		mediaRoot:       t.TempDir(),
		allowedPrefixes: map[string]struct{}{"mamuree": {}},
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected body to contain 'ok', got %q", body)
	}
}

// helper: 테스트용 미디어 루트 생성 + 파일 배치
func setupMedia(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	rel := "/mamuree/uploads/tasks/abc/file.txt"
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rel
}

func TestServeValidSignedFile(t *testing.T) {
	root, rel := setupMedia(t)
	cfg := serverConfig{
		signSecret:      "test-secret",
		mediaRoot:       root,
		allowedPrefixes: map[string]struct{}{"mamuree": {}},
	}
	h := newHandler(cfg)
	s := signer.New(cfg.signSecret)

	url := s.Sign("", rel, 10*time.Minute) // baseURL 빈 문자열 → "/mamuree/...?exp=&sig="
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "hello" {
		t.Errorf("body mismatch: want 'hello', got %q", body)
	}
}

func TestServeExpired(t *testing.T) {
	root, rel := setupMedia(t)
	cfg := serverConfig{signSecret: "test-secret", mediaRoot: root, allowedPrefixes: map[string]struct{}{"mamuree": {}}}
	h := newHandler(cfg)
	s := signer.New(cfg.signSecret)

	url := s.Sign("", rel, -1*time.Second)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestServeTamperedSig(t *testing.T) {
	root, rel := setupMedia(t)
	cfg := serverConfig{signSecret: "test-secret", mediaRoot: root, allowedPrefixes: map[string]struct{}{"mamuree": {}}}
	h := newHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, rel+"?exp=9999999999&sig=bad", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestServeMissingSignature(t *testing.T) {
	root, rel := setupMedia(t)
	cfg := serverConfig{signSecret: "test-secret", mediaRoot: root, allowedPrefixes: map[string]struct{}{"mamuree": {}}}
	h := newHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, rel, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestServeUnknownAppPrefix(t *testing.T) {
	root, _ := setupMedia(t)
	os.MkdirAll(filepath.Join(root, "unknown"), 0o755)
	os.WriteFile(filepath.Join(root, "unknown", "foo.txt"), []byte("secret"), 0o644)

	cfg := serverConfig{signSecret: "test-secret", mediaRoot: root, allowedPrefixes: map[string]struct{}{"mamuree": {}}}
	h := newHandler(cfg)
	s := signer.New(cfg.signSecret)

	url := s.Sign("", "/unknown/foo.txt", 10*time.Minute)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown app, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "UNKNOWN_APP") {
		t.Errorf("expected UNKNOWN_APP error code, got: %s", rec.Body.String())
	}
}

func TestServePathTraversal(t *testing.T) {
	root, _ := setupMedia(t)
	parent := filepath.Dir(root)
	secretPath := filepath.Join(parent, "secret.txt")
	_ = os.WriteFile(secretPath, []byte("topsecret"), 0o644)
	defer os.Remove(secretPath)

	cfg := serverConfig{signSecret: "test-secret", mediaRoot: root, allowedPrefixes: map[string]struct{}{"mamuree": {}}}
	h := newHandler(cfg)
	s := signer.New(cfg.signSecret)

	evilPath := "/mamuree/../secret.txt"
	url := s.Sign("", evilPath, 10*time.Minute)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("path traversal should be blocked, got 200 with body: %s", rec.Body.String())
	}
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Errorf("expected 403 or 404, got %d", rec.Code)
	}
}
