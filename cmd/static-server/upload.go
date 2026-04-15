package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	safeExtRe     = regexp.MustCompile(`^\.[A-Za-z0-9]{1,10}$`)
	safeSegmentRe = regexp.MustCompile(`^[A-Za-z0-9_\-]{1,64}$`)
)

// uploadResponse는 /upload 성공 응답 포맷이다.
type uploadResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (h *handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.maxUploadBytes)

	if err := r.ParseMultipartForm(h.cfg.maxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
			writeError(w, http.StatusRequestEntityTooLarge, "TOO_LARGE", "파일이 너무 큽니다")
			return
		}
		writeError(w, http.StatusBadRequest, "BAD_FORM", "multipart 파싱 실패")
		return
	}

	visibility := r.FormValue("visibility")
	if visibility != "public" && visibility != "private" {
		writeError(w, http.StatusBadRequest, "BAD_VISIBILITY", "visibility는 public 또는 private")
		return
	}
	app := r.FormValue("app")
	if _, ok := h.cfg.allowedPrefixes[app]; !ok {
		writeError(w, http.StatusBadRequest, "BAD_APP", "알 수 없는 app")
		return
	}

	file, fh, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "NO_FILE", "file 필드가 필요합니다")
		return
	}
	defer file.Close()

	ext := sanitizeExt(filepath.Ext(fh.Filename))
	id, err := generateID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "ID 생성 실패")
		return
	}

	prefix := r.FormValue("prefix")
	cleanedPrefix, ok := sanitizePrefix(prefix)
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_PREFIX", "prefix에 허용되지 않은 문자가 포함됨")
		return
	}

	relPath := buildRelPath(visibility, app, cleanedPrefix, id+ext)
	fullPath := filepath.Join(h.cfg.mediaRoot, relPath)
	rootClean := filepath.Clean(h.cfg.mediaRoot)
	if !strings.HasPrefix(filepath.Clean(fullPath), rootClean+string(os.PathSeparator)) {
		writeError(w, http.StatusBadRequest, "BAD_PATH", "경로가 올바르지 않습니다")
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "디렉토리 생성 실패")
		return
	}

	out, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "파일 생성 실패")
		return
	}
	written, copyErr := io.Copy(out, file)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(fullPath)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "파일 쓰기 실패")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(uploadResponse{Path: relPath, Size: written})
}

func sanitizeExt(ext string) string {
	if ext == "" {
		return ""
	}
	if !safeExtRe.MatchString(ext) {
		return ""
	}
	return strings.ToLower(ext)
}

// sanitizePrefix는 prefix의 각 세그먼트를 검증한다. 모두 허용 가능한 경우만 (cleaned, true) 반환.
// 빈 prefix도 유효한 입력이며 (cleaned="", true)로 반환된다.
func sanitizePrefix(prefix string) (string, bool) {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return "", true
	}
	segs := strings.Split(prefix, "/")
	for _, s := range segs {
		if !safeSegmentRe.MatchString(s) {
			return "", false
		}
	}
	return strings.Join(segs, "/"), true
}

func buildRelPath(visibility, app, prefix, filename string) string {
	parts := []string{visibility, app}
	if prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, filename)
	return "/" + strings.Join(parts, "/")
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
