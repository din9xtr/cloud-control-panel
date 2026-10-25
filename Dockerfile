# syntax=docker/dockerfile:1.7

FROM node:24.22.1-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY api /src/api
COPY web ./
RUN pnpm build

FROM golang:1.27-alpine AS server
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY api ./api
COPY internal ./internal
COPY --from=web /src/internal/web/dist ./internal/web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/cloud ./cmd/cloud
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/cloud /cloud
COPY --from=server --chown=65532:65532 /out/data /data
ENV CLOUD_ADDR=:8080 CLOUD_DATA_DIR=/data
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/cloud", "healthcheck"]
ENTRYPOINT ["/cloud"]
CMD ["serve"]
