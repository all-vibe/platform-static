package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/all-vibe/platform-static/pkg/signer"
)

func setupPublicFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServePublicNoSignature(t *testing.T) {
	h, root := newTestHandler(t)
	rel := "/public/orderflow/products/abc.jpg"
	setupPublicFile(t, root, rel, []byte("image-bytes"))

	// 서명 없이 접근해도 200
	req := httptest.NewRequest(http.MethodGet, rel, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "image-bytes" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestServePrivateStillRequiresSignature(t *testing.T) {
	h, root := newTestHandler(t)
	rel := "/private/orderflow/docs/secret.pdf"
	setupPublicFile(t, root, rel, []byte("secret"))

	// 서명 없이 접근 → 401
	req := httptest.NewRequest(http.MethodGet, rel, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}

	// 서명으로 접근 → 200
	s := signer.New("test-secret")
	signed := s.Sign("", rel, 10*time.Minute)
	req2 := httptest.NewRequest(http.MethodGet, signed, nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("want 200 with signature, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestServePublicUnknownApp(t *testing.T) {
	h, root := newTestHandler(t)
	// 허용되지 않은 app prefix → 서명 없어도 404
	setupPublicFile(t, root, "/public/unknown/a.jpg", []byte("x"))

	req := httptest.NewRequest(http.MethodGet, "/public/unknown/a.jpg", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestParseVisibilityPath(t *testing.T) {
	cases := []struct {
		in             string
		wantVisibility string
		wantApp        string
		wantOK         bool
	}{
		{"/public/orderflow/a.jpg", "public", "orderflow", true},
		{"/private/orderflow/a.pdf", "private", "orderflow", true},
		{"/mamuree/uploads/a.txt", "", "mamuree", true},
		{"/public/", "", "", false},
		{"/", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		v, a, ok := parseVisibilityPath(c.in)
		if v != c.wantVisibility || a != c.wantApp || ok != c.wantOK {
			t.Errorf("parseVisibilityPath(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, v, a, ok, c.wantVisibility, c.wantApp, c.wantOK)
		}
	}
}
