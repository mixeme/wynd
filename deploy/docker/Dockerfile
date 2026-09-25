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
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /wynd ./cmd/wynd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -S wynd \
	&& adduser -S -G wynd -h /data wynd

COPY --from=build /wynd /usr/local/bin/wynd

USER wynd
WORKDIR /data
ENV WYND_DATA_DIR=/data
# The binary defaults to loopback; inside the container the proxy is another host.
ENV WYND_LISTEN=:7676

EXPOSE 7676
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/wynd"]
