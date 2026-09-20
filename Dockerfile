# PhraseMate 网页实例。Windows 请用 Docker Desktop（Linux 容器模式）运行。
# 桌面窗口 / 托盘 / 置顶速记窗无法在容器中使用，请用浏览器打开 http://localhost:8080

FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/phrasemate .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S phrasemate \
    && adduser -S -G phrasemate -u 1000 phrasemate \
    && mkdir -p /data \
    && chown -R phrasemate:phrasemate /data

COPY --from=builder /out/phrasemate /usr/local/bin/phrasemate

ENV PHRASEMATE_WEB=1 \
    PHRASEMATE_ADDR=:8080 \
    PHRASEMATE_DB=/data/phrasemate.db \
    TZ=Asia/Shanghai

EXPOSE 8080
VOLUME ["/data"]

USER phrasemate
WORKDIR /data

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/status >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/phrasemate"]
