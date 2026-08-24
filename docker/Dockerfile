# ---------------------------------------------------------------------------
# Build the Vite UI
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:26-alpine AS web-builder

WORKDIR /src

COPY web/package.json web/package-lock.json ./web/
RUN --mount=type=cache,target=/root/.npm \
    npm --prefix web ci

COPY web/ ./web/
RUN npm --prefix web run build

# ---------------------------------------------------------------------------
# Compile the server
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS server-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY server/ ./server/

# Embed the UI built in stage 1
COPY --from=web-builder /src/server/pkg/modules/web/dist ./server/pkg/modules/web/dist

ARG TARGETOS
ARG TARGETARCH

# Stamped into the binary the same way goreleaser stamps the release archives, so
# `docker run <image> version` names the build it came from. They default to the
# placeholders a plain `go build` leaves, which is what a local build is.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /out/server ./server/cmd/server

# ---------------------------------------------------------------------------
# Setup runtime
# ---------------------------------------------------------------------------
FROM alpine:3.24 AS runtime

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 10001 -S cubby \
    && adduser -u 10001 -S -G cubby -h /app cubby

# /certs is where a TLS certificate and its private key are expected to be
# mounted from the host, read only. Nothing is put there by the image.
RUN mkdir -p /app /data /backups /certs \
    && chown -R cubby:cubby /app /data /backups \
    && chown cubby:cubby /certs && chmod 700 /certs

COPY --from=server-builder /out/server /usr/local/bin/server
COPY --chown=cubby:cubby docker/config.yaml /app/config.yaml

ENV CUBBY_ENV=prod \
    CUBBY_DATA_DIR=/data \
    CUBBY_BACKUP_DIR=/backups \
    CUBBY_API_BIND_IP=0.0.0.0 \
    CUBBY_API_PORT=8080

WORKDIR /app
USER cubby:cubby

EXPOSE 8080

ENTRYPOINT ["server"]
CMD ["start"]
