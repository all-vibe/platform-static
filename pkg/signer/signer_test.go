package signer

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func parseSignedURL(t *testing.T, signedURL string) (int64, string) {
	t.Helper()
	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	expStr := u.Query().Get("exp")
	sig := u.Query().Get("sig")
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		t.Fatalf("parse exp: %v", err)
	}
	return exp, sig
}

func TestSignerSignVerify(t *testing.T) {
	s := New("test-secret")
	path := "/mamuree/uploads/tasks/abc/file.jpg"

	signedURL := s.Sign("https://static.allvibe.ai", path, 10*time.Minute)
	if signedURL == "" {
		t.Fatal("Sign returned empty URL")
	}

	// URL 포맷 회귀 방지: /api/files prefix가 절대 포함되지 않아야 함
	if strings.Contains(signedURL, "/api/files") {
		t.Errorf("Sign URL should not contain /api/files prefix, got: %s", signedURL)
	}

	// baseURL + path가 정확히 이어져야 함
	expectedPrefix := "https://static.allvibe.ai/mamuree/uploads/tasks/abc/file.jpg?"
	if !strings.HasPrefix(signedURL, expectedPrefix) {
		t.Errorf("Sign URL should start with %q, got: %s", expectedPrefix, signedURL)
	}

	exp, sig := parseSignedURL(t, signedURL)
	if !s.Verify(path, exp, sig) {
		t.Error("Verify failed for valid signature")
	}
}

func TestSignerExpired(t *testing.T) {
	s := New("test-secret")
	path := "/mamuree/uploads/tasks/abc/file.jpg"
	signedURL := s.Sign("https://static.allvibe.ai", path, -1*time.Second)
	exp, sig := parseSignedURL(t, signedURL)
	if s.Verify(path, exp, sig) {
		t.Error("Verify should fail for expired signature")
	}
}

func TestSignerTampered(t *testing.T) {
	s := New("test-secret")
	path := "/mamuree/uploads/tasks/abc/file.jpg"
	signedURL := s.Sign("https://static.allvibe.ai", path, 10*time.Minute)
	exp, _ := parseSignedURL(t, signedURL)
	if s.Verify(path, exp, "tamperedsig") {
		t.Error("Verify should fail for tampered signature")
	}
}

func TestSignerWrongPath(t *testing.T) {
	s := New("test-secret")
	path := "/mamuree/uploads/tasks/abc/file.jpg"
	signedURL := s.Sign("https://static.allvibe.ai", path, 10*time.Minute)
	exp, sig := parseSignedURL(t, signedURL)
	if s.Verify("/mamuree/uploads/tasks/abc/other.jpg", exp, sig) {
		t.Error("Verify should fail for different path")
	}
}

func TestFileSignTTLConstant(t *testing.T) {
	if FileSignTTL != 10*time.Minute {
		t.Errorf("FileSignTTL should be 10 minutes, got %v", FileSignTTL)
	}
}
