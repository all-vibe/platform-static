// Command static-server serves signed static files for the allvibe platform.
package main

import (
	"fmt"
	"net/http"
	"os"
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
	// 파일 서빙은 다음 Task에서 구현
	http.NotFound(w, r)
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
