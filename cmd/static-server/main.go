// Command static-server serves signed static files for the allvibe platform.
package main

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/all-vibe/platform-static/pkg/signer"
)

type serverConfig struct {
	signSecret      string
	mediaRoot       string
	allowedPrefixes map[string]struct{}
	port            string
}

type handler struct {
	cfg    serverConfig
	signer *signer.Signer
}

func newHandler(cfg serverConfig) *handler {
	return &handler{cfg: cfg, signer: signer.New(cfg.signSecret)}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	h.serveFile(w, r)
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	// 서명 검증
	expStr := r.URL.Query().Get("exp")
	sig := r.URL.Query().Get("sig")
	if expStr == "" || sig == "" {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "서명이 누락되었습니다")
		return
	}
	var exp int64
	if _, err := fmt.Sscanf(expStr, "%d", &exp); err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "유효하지 않은 서명")
		return
	}
	if !h.signer.Verify(urlPath, exp, sig) {
		writeError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "유효하지 않은 서명")
		return
	}

	// 앱 prefix 화이트리스트 체크
	segs := strings.SplitN(strings.TrimPrefix(urlPath, "/"), "/", 2)
	if len(segs) == 0 || segs[0] == "" {
		writeError(w, http.StatusNotFound, "UNKNOWN_APP", "알 수 없는 경로")
		return
	}
	if _, ok := h.cfg.allowedPrefixes[segs[0]]; !ok {
		writeError(w, http.StatusNotFound, "UNKNOWN_APP", "알 수 없는 앱")
		return
	}

	// 실제 파일 경로
	fullPath := filepath.Join(h.cfg.mediaRoot, urlPath)

	// Path traversal 방어
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

	// Content-Disposition: download=1이면 attachment, 아니면 inline.
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

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":%q,"message":%q}}`, code, message)
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

func main() {
	cfg := loadConfig()
	h := newHandler(cfg)
	addr := ":" + cfg.port
	fmt.Fprintf(os.Stderr, "static-server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() serverConfig {
	secret := os.Getenv("FILE_SIGN_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "FATAL: FILE_SIGN_SECRET not set")
		os.Exit(1)
	}
	root := os.Getenv("MEDIA_ROOT")
	if root == "" {
		root = "/media"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	prefixesCSV := os.Getenv("ALLOWED_APP_PREFIXES")
	if prefixesCSV == "" {
		fmt.Fprintln(os.Stderr, "FATAL: ALLOWED_APP_PREFIXES not set (e.g. 'mamuree')")
		os.Exit(1)
	}
	prefixes := make(map[string]struct{})
	for _, p := range strings.Split(prefixesCSV, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			prefixes[p] = struct{}{}
		}
	}
	return serverConfig{
		signSecret:      secret,
		mediaRoot:       root,
		allowedPrefixes: prefixes,
		port:            port,
	}
}
