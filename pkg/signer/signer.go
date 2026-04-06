// Package signer provides HMAC-SHA256 signed URL generation and verification
// for allvibe platform static file serving.
package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// FileSignTTL은 서명된 파일 URL의 기본 유효 시간이다.
const FileSignTTL = 10 * time.Minute

// Signer는 HMAC-SHA256 기반의 URL 서명자이다.
type Signer struct {
	secret []byte
}

// New는 주어진 비밀 문자열로 Signer를 생성한다.
func New(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

// Sign은 baseURL + path에 exp/sig 쿼리를 붙인 서명 URL을 반환한다.
// baseURL 예: "https://static.allvibe.ai"
// path 예:    "/mamuree/uploads/tasks/abc/file.jpg"
// 반환 URL:   "https://static.allvibe.ai/mamuree/uploads/tasks/abc/file.jpg?exp=...&sig=..."
func (s *Signer) Sign(baseURL, path string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	mac := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(mac, "%s:%d", path, exp)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s%s?exp=%d&sig=%s", baseURL, path, exp, sig)
}

// Verify는 주어진 path와 exp, sig로 서명의 유효성을 검증한다.
// exp이 과거이면 거부한다.
func (s *Signer) Verify(path string, exp int64, sig string) bool {
	if time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(mac, "%s:%d", path, exp)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
