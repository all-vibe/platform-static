# platform-static

Allvibe 플랫폼 공용 파일 서비스. 모든 서비스의 파일 업로드 요청을 받아 저장하고, 서명 URL 발급과 공개/비공개 서빙을 책임진다.

## 구성

- `pkg/signer` — HMAC-SHA256(path:exp) 서명/검증 공용 패키지
- `cmd/static-server` — 표준 라이브러리 기반 HTTP 서버
- `Dockerfile` — multi-stage build, alpine 최종 이미지

## 경로 규칙

- `/public/{app}/...` — 공개 파일. 서명 없이 `GET` 가능 (브라우저 이미지용)
- `/private/{app}/...` — 비공개 파일. 서명된 URL로만 `GET` 가능 (첨부파일용)
- `/{app}/...` — legacy. 서명 필수 (mamuree 기존 경로 호환)

## API

### `POST /upload`

- 인증: `Authorization: Bearer $UPLOAD_AUTH_TOKEN`
- Body: `multipart/form-data`
  - `file` — 파일
  - `visibility` — `public` 또는 `private`
  - `app` — `ALLOWED_APP_PREFIXES`에 포함된 앱 이름
  - `prefix` (optional) — 하위 경로 힌트. `[A-Za-z0-9_-]` 만 허용, `/`로 분리
- 성공 응답: `{ "path": "/{visibility}/{app}/{prefix?}/{uuid}.{ext}", "size": <bytes> }`
- 최대 크기: `MAX_UPLOAD_SIZE_MB` (기본 50MB) — 초과 시 `413`

### `POST /sign`

- 인증: `Authorization: Bearer $UPLOAD_AUTH_TOKEN`
- Body: `application/json`
  - `paths` — `/private/...` 경로 배열 (최대 200개). `/public/*` 포함 시 `400`
  - `ttl` (optional) — 초 단위. 기본 `FileSignTTL` (10분), 범위 `[1, 3600]`
- 성공 응답: `{ "urls": { "<path>": "<signed URL>" } }`

### `GET /{...}`

- `/public/*` — 서명 없이 서빙
- 그 외 — `?exp=<unix>&sig=<hex>` 필수
- `?download=1` — `Content-Disposition: attachment`
- `?name=<url-encoded>` — RFC 5987 filename*

## 환경변수

| 변수 | 필수 | 설명 |
|---|---|---|
| `FILE_SIGN_SECRET` | ✅ | 서명 HMAC secret |
| `UPLOAD_AUTH_TOKEN` | ✅ | 업로드/서명 API bearer 토큰 |
| `PUBLIC_BASE_URL` | ✅ | 서명 URL 반환 시 사용할 base (예: `https://static.allvibe.ai`) |
| `ALLOWED_APP_PREFIXES` | ✅ | 허용 앱 이름 CSV (예: `mamuree,orderflow`) |
| `MEDIA_ROOT` | | 파일 저장 루트. 기본 `/media` |
| `MAX_UPLOAD_SIZE_MB` | | 업로드 최대 크기. 기본 `50` |
| `PORT` | | HTTP 포트. 기본 `8080` |
