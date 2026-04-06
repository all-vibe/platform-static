# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY pkg/ ./pkg/
COPY cmd/ ./cmd/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/static-server ./cmd/static-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 65532 -g 65532 app
USER 65532:65532
COPY --from=builder /out/static-server /usr/local/bin/static-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/static-server"]
