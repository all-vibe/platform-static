package main

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	visibility, app, ok := parseVisibilityPath(urlPath)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "알 수 없는 경로")
		return
	}

	// /public/*는 서명 생략. /private/*와 legacy /{app}/*는 서명 필수.
	if visibility != "public" {
		if !h.verifySignature(w, r, urlPath) {
			return
		}
	}

	if _, ok := h.cfg.allowedPrefixes[app]; !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_APP", "알 수 없는 앱")
		return
	}

	fullPath := filepath.Join(h.cfg.mediaRoot, urlPath)
	cleaned := filepath.Clean(fullPath)
	rootClean := filepath.Clean(h.cfg.mediaRoot)
	if !strings.HasPrefix(cleaned, rootClean+string(os.PathSeparator)) && cleaned != rootClean {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "접근 불가")
		return
	}
	fullPath = cleaned

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "파일을 찾을 수 없습니다")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "서버 오류")
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "서버 오류")
		return
	}

	if ct := mime.TypeByExtension(filepath.Ext(fullPath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	dispType := "inline"
	if r.URL.Query().Get("download") == "1" {
		dispType = "attachment"
	}
	rawName := r.URL.Query().Get("name")
	if rawName != "" {
		safeName := stripCRLF(rawName)
		encoded := urlPercentEncode(safeName)
		w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", dispType, encoded))
	} else {
		w.Header().Set("Content-Disposition", dispType)
	}

	http.ServeContent(w, r, filepath.Base(fullPath), stat.ModTime(), f)
}

func (h *handler) verifySignature(w http.ResponseWriter, r *http.Request, urlPath string) bool {
	expStr := r.URL.Query().Get("exp")
	sig := r.URL.Query().Get("sig")
	if expStr == "" || sig == "" {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "서명이 누락되었습니다")
		return false
	}
	var exp int64
	if _, err := fmt.Sscanf(expStr, "%d", &exp); err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "유효하지 않은 서명")
		return false
	}
	if !h.signer.Verify(urlPath, exp, sig) {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "유효하지 않은 서명")
		return false
	}
	return true
}

// parseVisibilityPath는 URL path를 (visibility, app)로 파싱한다.
// /public/{app}/... → ("public", app, true)
// /private/{app}/... → ("private", app, true)
// /{app}/... → ("", app, true)   — 기존 경로 호환
func parseVisibilityPath(urlPath string) (visibility, app string, ok bool) {
	trimmed := strings.TrimPrefix(urlPath, "/")
	if trimmed == "" {
		return "", "", false
	}
	first := strings.SplitN(trimmed, "/", 2)
	if first[0] == "public" || first[0] == "private" {
		if len(first) < 2 || first[1] == "" {
			return "", "", false
		}
		rest := strings.SplitN(first[1], "/", 2)
		if rest[0] == "" {
			return "", "", false
		}
		return first[0], rest[0], true
	}
	if first[0] == "" {
		return "", "", false
	}
	return "", first[0], true
}

// stripCRLF는 HTTP 헤더 인젝션 방지를 위해 CR/LF 및 제어 문자를 제거한다.
func stripCRLF(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\r' || r == '\n' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// urlPercentEncode는 RFC 5987 filename*용 percent encoding을 수행한다.
func urlPercentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '!' || c == '#' || c == '$' || c == '&' || c == '+' || c == '-' ||
			c == '.' || c == '^' || c == '_' || c == '`' || c == '|' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
