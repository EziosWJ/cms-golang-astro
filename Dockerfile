# syntax=docker/dockerfile:1

FROM node:24.16.0-alpine AS admin-build
WORKDIR /src/cms-admin
COPY cms-admin/package.json cms-admin/package-lock.json ./
RUN npm ci
COPY cms-admin/ ./
RUN npm run build

FROM golang:1.26.5-alpine AS go-build
WORKDIR /src/cms-api
RUN apk add --no-cache gcc musl-dev
COPY cms-api/go.mod cms-api/go.sum ./
RUN go mod download
COPY cms-api/ ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/backup ./cmd/backup

FROM go-build AS api-build
COPY --from=admin-build /src/cms-admin/dist ./internal/webui/dist
RUN CGO_ENABLED=1 go build -tags=embedweb -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22 AS runtime
RUN addgroup -S app && adduser -S -G app app \
  && mkdir -p /data/cms/uploads \
  && chown -R app:app /data/cms
WORKDIR /app
COPY --from=go-build /src/cms-api/configs/config.yaml ./configs/config.yaml
COPY --from=go-build /src/cms-api/configs/config.sqlite.yaml ./configs/config.sqlite.yaml
ENV APP_ENV=prod \
    APP_CONFIG_PROFILE=sqlite \
    APP_DATABASE__URL=/data/cms/cms.db \
    APP_FILE__STORAGE_ROOT=/data/cms/uploads \
    APP_HTTP__ADDRESS=:8080
USER app

FROM runtime AS migrate
COPY --from=go-build /out/migrate ./migrate
COPY --from=go-build /src/cms-api/migrations ./migrations
ENTRYPOINT ["/app/migrate"]

FROM runtime AS backup
COPY --from=go-build /out/backup ./backup
ENTRYPOINT ["/app/backup"]

FROM runtime AS api
COPY --from=api-build /out/api ./api
EXPOSE 8080
ENTRYPOINT ["/app/api"]
