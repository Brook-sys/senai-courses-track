# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/senai-courses-track \
    ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && install -d -o app -g app /data

COPY --from=build /out/senai-courses-track /usr/local/bin/senai-courses-track

USER app
WORKDIR /data

ENV SENAI_TRACK_ADDR=:8020 \
    SENAI_TRACK_DB_PATH=/data/courses.db \
    TZ=America/Sao_Paulo

EXPOSE 8020
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8020/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/senai-courses-track"]
