FROM node:22-alpine AS web

WORKDIR /src
COPY VERSION ./
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
RUN cd web && npm run build

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
# -trimpath убирает пути сборочной машины из бинаря (DEP-3).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /wynd ./cmd/wynd

FROM alpine:3.21

# Версию передаёт сборщик: docker build --build-arg VERSION=$(cat VERSION).
ARG VERSION=dev

LABEL org.opencontainers.image.title="Wynd" \
	org.opencontainers.image.description="Private circle journal" \
	org.opencontainers.image.version="$VERSION" \
	org.opencontainers.image.source="https://github.com/mixeme/wynd" \
	org.opencontainers.image.url="https://github.com/mixeme/wynd" \
	org.opencontainers.image.licenses="AGPL-3.0-or-later"

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S wynd \
	&& adduser -S -G wynd -h /data wynd

COPY --from=build /wynd /usr/local/bin/wynd
# AGPL §4: копия лицензии едет вместе с бинарём (LIC-5).
COPY LICENSE /usr/share/licenses/wynd/LICENSE

USER wynd
WORKDIR /data
ENV WYND_DATA_DIR=/data
# The binary defaults to loopback; inside the container the proxy is another host.
ENV WYND_LISTEN=:7676

EXPOSE 7676
VOLUME ["/data"]

# busybox wget есть в alpine; /health отвечает без базы и без сессии.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
	CMD wget -q -O /dev/null http://127.0.0.1:7676/health || exit 1

ENTRYPOINT ["/usr/local/bin/wynd"]
