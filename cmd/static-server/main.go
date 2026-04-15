// Command static-server serves signed static files for the allvibe platform.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/all-vibe/platform-static/pkg/signer"
)

type serverConfig struct {
	signSecret       string
	apiToken         string
	mediaRoot        string
	publicBaseURL    string
	allowedPrefixes  map[string]struct{}
	maxUploadBytes   int64
	port             string
}

type handler struct {
	cfg    serverConfig
	signer *signer.Signer
}

func newHandler(cfg serverConfig) *handler {
	return &handler{cfg: cfg, signer: signer.New(cfg.signSecret)}
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
	secret := mustEnv("FILE_SIGN_SECRET")
	token := mustEnv("UPLOAD_AUTH_TOKEN")
	publicBaseURL := mustEnv("PUBLIC_BASE_URL")
	prefixesCSV := mustEnv("ALLOWED_APP_PREFIXES")

	root := envOr("MEDIA_ROOT", "/media")
	port := envOr("PORT", "8080")
	maxMB, err := strconv.Atoi(envOr("MAX_UPLOAD_SIZE_MB", "50"))
	if err != nil || maxMB <= 0 {
		maxMB = 50
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
		apiToken:        token,
		mediaRoot:       root,
		publicBaseURL:   strings.TrimRight(publicBaseURL, "/"),
		allowedPrefixes: prefixes,
		maxUploadBytes:  int64(maxMB) * 1024 * 1024,
		port:            port,
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "FATAL: %s not set\n", key)
		os.Exit(1)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
