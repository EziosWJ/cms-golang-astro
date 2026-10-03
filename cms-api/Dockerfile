# syntax=docker/dockerfile:1

FROM node:24.16.0-alpine AS frontend

WORKDIR /web
COPY cms-admin/package.json cms-admin/package-lock.json ./
RUN npm ci
COPY cms-admin/ ./
RUN npm run build

FROM golang:1.26.5-alpine AS build

WORKDIR /src

RUN apk add --no-cache gcc musl-dev

COPY cms-api/go.mod cms-api/go.sum ./
RUN go mod download

COPY cms-api/ .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/backup ./cmd/backup

FROM build AS api-build

COPY --from=frontend /web/dist ./internal/webui/dist
RUN CGO_ENABLED=1 go build -tags=embedweb -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY cms-api/configs ./configs
RUN mkdir -p /data/cms/uploads && chown -R app:app /data/cms
ENV APP_ENV=prod \
    APP_CONFIG_PROFILE=sqlite \
    APP_DATABASE__URL=/data/cms/cms.db \
    APP_FILE__STORAGE_ROOT=/data/cms/uploads \
    APP_HTTP__ADDRESS=:8080
USER app

FROM runtime AS migrate

COPY --from=build /out/migrate ./migrate
COPY cms-api/migrations ./migrations
ENTRYPOINT ["/app/migrate"]

FROM runtime AS backup

COPY --from=build /out/backup ./backup
ENTRYPOINT ["/app/backup"]

# Keep API last so a plain `docker build` produces the application image.
FROM runtime AS api

COPY --from=api-build /out/api ./api
EXPOSE 8080
ENTRYPOINT ["/app/api"]
