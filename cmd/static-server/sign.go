package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/all-vibe/platform-static/pkg/signer"
)

type signRequest struct {
	Paths []string `json:"paths"`
	TTL   int      `json:"ttl"`
}

type signResponse struct {
	URLs map[string]string `json:"urls"`
}

const (
	minSignTTL = 1
	maxSignTTL = 3600
	maxSignPaths = 200
)

func (h *handler) handleSign(w http.ResponseWriter, r *http.Request) {
	var req signRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "JSON 파싱 실패")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "NO_PATHS", "paths가 비어있습니다")
		return
	}
	if len(req.Paths) > maxSignPaths {
		writeError(w, http.StatusBadRequest, "TOO_MANY_PATHS", "paths가 너무 많습니다")
		return
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = int(signer.FileSignTTL.Seconds())
	}
	if ttl < minSignTTL {
		ttl = minSignTTL
	}
	if ttl > maxSignTTL {
		ttl = maxSignTTL
	}

	urls := make(map[string]string, len(req.Paths))
	for _, p := range req.Paths {
		if !strings.HasPrefix(p, "/private/") {
			writeError(w, http.StatusBadRequest, "BAD_PATH", "private 경로만 서명 가능합니다: "+p)
			return
		}
		urls[p] = h.signer.Sign(h.cfg.publicBaseURL, p, time.Duration(ttl)*time.Second)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(signResponse{URLs: urls})
}
