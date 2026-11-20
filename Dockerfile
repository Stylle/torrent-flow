# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY internal/ ./internal/
COPY --from=frontend /src/frontend/dist/client ./frontend/dist/client
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /flow .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates su-exec && addgroup -g 10001 flow && adduser -D -H -u 10001 -G flow flow
COPY --from=build /flow /app/flow
COPY docker-entrypoint.sh /app/entrypoint.sh
COPY LICENSE README.md THIRD-PARTY-NOTICES.md /app/docs/
COPY licenses/ /app/docs/licenses/
LABEL org.opencontainers.image.title="Torrent Flow" org.opencontainers.image.version="0.1.7" org.opencontainers.image.licenses="MIT" org.opencontainers.image.source="https://github.com/Stylle/torrent-flow" org.opencontainers.image.documentation="https://github.com/Stylle/torrent-flow#readme" org.opencontainers.image.description="qBittorrent and Transmission traffic accounting with guarded per-torrent upload allocation"
RUN chmod 755 /app/entrypoint.sh
ENV DATA_DIR=/data LISTEN_ADDR=:8080 EXTERNAL_GUARDIAN=true
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/app/entrypoint.sh"]
