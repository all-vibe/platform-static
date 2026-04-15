package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignBatchSuccess(t *testing.T) {
	h, _ := newTestHandler(t)

	body := bytes.NewBufferString(`{"paths":["/private/orderflow/a.pdf","/private/orderflow/b.pdf"],"ttl":300}`)
	req := httptest.NewRequest(http.MethodPost, "/sign", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp signResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.URLs) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(resp.URLs))
	}
	for p, u := range resp.URLs {
		if !strings.HasPrefix(u, "https://static.example.com"+p+"?exp=") {
			t.Errorf("unexpected url for %s: %s", p, u)
		}
		if !strings.Contains(u, "&sig=") {
			t.Errorf("missing sig: %s", u)
		}
	}
}

func TestSignNoBearer(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sign", bytes.NewBufferString(`{"paths":["/private/x/y"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestSignRejectsPublicPaths(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sign", bytes.NewBufferString(`{"paths":["/public/orderflow/a.jpg"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "BAD_PATH") {
		t.Errorf("expected BAD_PATH: %s", rec.Body.String())
	}
}

func TestSignEmptyPaths(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/sign", bytes.NewBufferString(`{"paths":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestSignTTLClamped(t *testing.T) {
	h, _ := newTestHandler(t)
	// ttl > maxSignTTL → clamp to 3600
	body := bytes.NewBufferString(`{"paths":["/private/orderflow/a.pdf"],"ttl":99999}`)
	req := httptest.NewRequest(http.MethodPost, "/sign", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}
