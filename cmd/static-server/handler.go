package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/upload":
		if !h.requireBearer(w, r) {
			return
		}
		h.handleUpload(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/sign":
		if !h.requireBearer(w, r) {
			return
		}
		h.handleSign(w, r)
	case r.Method == http.MethodGet:
		h.serveFile(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "허용되지 않은 요청")
	}
}

// requireBearer는 Authorization: Bearer <token> 헤더를 상수시간으로 검증한다.
func (h *handler) requireBearer(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "인증 필요")
		return false
	}
	got := strings.TrimPrefix(auth, prefix)
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.cfg.apiToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "인증 실패")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%q,"message":%q}}`, code, message)
}
