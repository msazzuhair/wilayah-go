# Area Auto-Sync Services

[![Build and Publish Docker Images](https://github.com/msazzuhair/wilayah-go/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/msazzuhair/wilayah-go/actions/workflows/docker-publish.yml)
![Go Version](https://img.shields.io/badge/go-1.26-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)
![Docker](https://img.shields.io/badge/docker-ready-blue.svg)

This project provides two Go-based services to automatically synchronize Indonesian administrative region data from GitHub into a PostgreSQL database.

## Services

1.  **Simple Sync Service**: Synchronizes basic hierarchical data (`kode`, `nama`) for all levels (provinces, regencies, districts, villages).
2.  **Complex Sync Service**: Synchronizes detailed data for provinces and regencies, including capitals, astronomical locations (lat/lng), elevation, population, and area boundaries (polygon).

## Features

- **Auto-Sync:** Checks for new commits on GitHub daily and updates the database if changes are detected.
- **Relational Schema:** Splits the hierarchical data into dedicated tables.
- **Configurable:** All table names and database connection details are customizable via environment variables.
- **Dockerized:** Separate Dockerfiles for each service.
- **CI/CD:** GitHub Actions workflow to build and publish images to GHCR.

## Configuration

| Variable          | Description                  | Default (Simple) | Default (Complex)       |
|-------------------|------------------------------|------------------|-------------------------|
| `DB_DSN`          | PostgreSQL connection string | `postgres://...` | `postgres://...`        |
| `SOURCE_URL`      | SQL Source URL               | `wilayah.sql`    | `wilayah_level_1_2.sql` |
| `TABLE_PROVINCES` | Table name for provinces     | `provinces`      | `provinces`             |
| `TABLE_REGENCIES` | Table name for regencies     | `regencies`      | `regencies`             |
| `TABLE_DISTRICTS` | Table name for districts     | `districts`      | `districts`             |
| `TABLE_VILLAGES`  | Table name for villages      | `villages`       | `villages`              |
| `CRON_SCHEDULE`   | Cron expression              | `0 0 * * *`      | `0 0 * * *`             |

## How to Run

### Using Go

**Simple Sync:**
```bash
go run ./cmd/simple-sync/main.go
```

**Complex Sync:**
```bash
go run ./cmd/complex-sync/main.go
```

### Using Docker

**Simple Sync:**
```bash
docker build -t area-sync-simple -f simple.Dockerfile .
docker run --env-file .env area-sync-simple
```

**Complex Sync:**
```bash
docker build -t area-sync-complex -f complex.Dockerfile .
docker run --env-file .env area-sync-complex
```

## Deployment

The project includes a GitHub Actions workflow that automatically builds and pushes images to `ghcr.io` on every push to the `master` or `main` branch.

- `ghcr.io/msazzuhair/wilayah-go-simple:latest`
- `ghcr.io/msazzuhair/wilayah-go-complex:latest`

## License
MIT
