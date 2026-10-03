# CMS API

Go + Gin + GORM backend for the self hosted CMS. Existing system modules and `/api/system/*` routes retain their behavior. CMS domain APIs are not part of Phase 0. Database schema and seed data are managed by explicit Goose migrations; API startup never migrates the database.

## Requirements

- Go 1.26
- Node.js is needed only when building the embedded Admin assets; the running API binary does not need Node.

## Local SQLite

Run commands from `cms-api/`. The default database is `../.runtime/cms.db`, resolved from this working directory. The database helper creates its parent directory. Before first API startup, explicitly apply migrations:

```sh
APP_ENV=dev APP_JWT__SECRET='replace-with-a-local-secret' go run ./cmd/migrate up --kind all
APP_ENV=dev APP_JWT__SECRET='replace-with-a-local-secret' APP_FILE__STORAGE_ROOT="$PWD/../.runtime/uploads" go run ./cmd/api
```

`APP_JWT__SECRET` can be set in the environment or in an ignored local `configs/config.dev.yaml`. The checked-in dev template uses an explicit development-only value. Never reuse it in production. Configure file storage with an absolute path using `APP_FILE__STORAGE_ROOT` or local config.

To use PostgreSQL instead of the default SQLite profile, copy `configs/config.postgres.example.yaml` to the ignored `configs/config.postgres.yaml`, fill in the local credentials, and set `APP_CONFIG_PROFILE=postgres` for both explicit migration and API startup:

```sh
cp configs/config.postgres.example.yaml configs/config.postgres.yaml
# Edit the URL, username, and password in configs/config.postgres.yaml.
APP_ENV=dev APP_CONFIG_PROFILE=postgres APP_JWT__SECRET='replace-with-a-local-secret' go run ./cmd/migrate up --kind all
APP_ENV=dev APP_CONFIG_PROFILE=postgres APP_JWT__SECRET='replace-with-a-local-secret' APP_FILE__STORAGE_ROOT="$PWD/../.runtime/uploads" go run ./cmd/api
```

`APP_ENV` defaults to `dev`. Swagger routes are available only when `APP_ENV=dev` and `swagger.enabled=true`. The production config must disable Swagger; production also rejects placeholder or fewer than 32 character JWT secrets.

## PostgreSQL

PostgreSQL remains supported. Create an ignored `configs/config.dev.yaml` with `database.driver: postgres`, a PostgreSQL URL, username and password, then use the same explicit migration and API commands. The production example in `configs/config.prod.example.yaml` documents PostgreSQL settings.

## Embedded Admin

Build `cms-admin` first, then copy its production `dist` directory to `internal/webui/dist` and compile with the `embedweb` build tag:

```sh
npm --prefix ../cms-admin ci
npm --prefix ../cms-admin run build
rm -rf internal/webui/dist
cp -R ../cms-admin/dist internal/webui/dist
go build -tags=embedweb -o ../bin/cms-api ./cmd/api
```

The resulting binary serves the embedded Admin and API from one process. Development uses the Admin's Vite server separately.

## Docker

The Dockerfile expects the repository root as build context:

```sh
docker build -f cms-api/Dockerfile --target api -t cms-api ..
```

The API image includes the Admin build embedded in the Go binary. Provide runtime configuration and persistent storage through environment variables or mounted config/data paths.

## Checks

```sh
go test ./...
go vet ./...
```

PostgreSQL and SQLite integration tests live under `integration/` and are tagged `integration`. PostgreSQL integration tests require Docker. The default API and migration commands use SQLite; no automatic migration runs on startup.
