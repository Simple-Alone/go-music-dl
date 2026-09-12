# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

WORKDIR /app
ARG TARGETOS=linux
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=${TARGETARCH:-$(go env GOARCH)} go build -o music-dl ./cmd/music-dl

# Runtime stage
FROM alpine:3.22

ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="go-music-dl" \
      org.opencontainers.image.description="Modified go-music-dl web and CLI distribution" \
      org.opencontainers.image.source="https://github.com/Simple-Alone/go-music-dl" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.revision="${VCS_REF}"

RUN apk --no-cache add ca-certificates tzdata ffmpeg \
    && ffmpeg -version >/dev/null \
    && ffprobe -version >/dev/null

ENV TZ=Asia/Shanghai

RUN adduser -D -s /bin/sh appuser

WORKDIR /home/appuser/

COPY --from=builder /app/music-dl .
COPY LICENSE NOTICE THIRD_PARTY_NOTICES.md /usr/share/licenses/go-music-dl/
RUN chown -R appuser:appuser /home/appuser/ \
    && chmod 0644 /usr/share/licenses/go-music-dl/*

USER appuser

EXPOSE 8080

CMD ["./music-dl", "web", "--port", "8080", "--no-browser"]
