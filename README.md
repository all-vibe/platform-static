# platform-static

Allvibe 플랫폼 공용 정적 파일 서빙 인프라. HMAC 서명 URL 검증과 `/data/media/<app>/...` 파일 서빙을 담당하는 경량 Go 바이너리 `static-server`와, 이 바이너리와 각 앱 backend가 공유하는 `pkg/signer` 패키지를 담는다.

## 구성

- `pkg/signer` — HMAC-SHA256(path:exp) 서명/검증 공용 패키지
- `cmd/static-server` — 표준 라이브러리 기반 HTTP 파일 서버 (~150 LOC)
- `Dockerfile` — multi-stage build, distroless-style alpine 최종 이미지
