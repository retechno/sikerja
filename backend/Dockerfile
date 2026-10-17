# syntax=docker/dockerfile:1
# Image produksi: binary statis (CGO off) di atas distroless. Target: `api` (juga memuat migrate & seed) dan `worker`.
#   docker build --target api    -t sikerja-api    ./backend
#   docker build --target worker -t sikerja-worker ./backend

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    for c in api worker migrate seed; do go build -ldflags="-s -w" -o /out/$c ./cmd/$c || exit 1; done
RUN mkdir -p /out/storage

FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=build /out/api /out/migrate /out/seed /app/
# folder lampiran: dimiliki user nonroot (volume bernama mewarisi kepemilikan ini saat pertama dibuat)
COPY --from=build --chown=nonroot:nonroot /out/storage /app/storage
ENV APP_ENV=production HTTP_ADDR=:8080 STORAGE_DIR=/app/storage
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=40s --retries=5 CMD ["/app/api", "-healthcheck"]
ENTRYPOINT ["/app/api"]

FROM gcr.io/distroless/static-debian12:nonroot AS worker
WORKDIR /app
COPY --from=build /out/worker /app/worker
ENV APP_ENV=production
ENTRYPOINT ["/app/worker"]
