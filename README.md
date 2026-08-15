# Cubby

Extremely simple and light self-hosted workspace for daily stuff: recurring tasks, shopping lists, recipes and routine tracking.

## Motivation

My main goal was to play around and explore some simple but interesting things like:

- Autocomplete suggestions using [Bluge](https://github.com/blugelabs/bluge) indexing, with LRU handler rotation and idle monitoring.
- A centralized freeze gateway with the power of stopping and resuming the entire application.
- A simple backup and restore strategy.
- Embedded UI with Vite.
- Bruno API collection fully generated and managed with AI
- *_Embedded UI, most of the CRUDs and tests are fully generated as well._

## Constraints

This application is designed as a simple, self-hosted, 'share-with-family' type of service.
It avoids complex optimizations where they wouldn't be fun to build or where they would introduce unnecessary complexity.
The focus here is to be simple and light.

## Features

This service is a set of random simple features based on my daily routine. The main goal is to create something I'll use every day, running locally on my Raspberry Pi.

- **Tasks board**: recurring tasks (`once`, `daily`, `weekly`, `monthly`, `annual`) displayed in a simple day-to-day board. Designed to track what needs to be done and how to do it.
- **Recipes**: simple recipes organizer.
- **Shopping lists**: remember what to buy, create a list from recipes.
- **Routine goals**: weekday-scheduled routines with a per-month grid of marked days and a point value per routine.
- **Workspaces**: every task, list, recipe and routine belongs to a workspace. Workspaces can be shared with other users.
- **Backups**: zip archives for creating and restoring the system data.
- **First-run setup**: a fresh deployment generates its own signing secret and prints a one-time token, which is what allows bootstraping the system.

## Todo

- A proper db migration mechanism.
- Dashboards for routine goals.
- Notification system.

## Prerequisites

| Requirement   | Version | Needed for                               |
| ------------- | ------- | ---------------------------------------- |
| Go            | 1.26+   | building and running the server          |
| Node.js + npm | 22.x    | building the embedded UI (`go generate`) |

## Installation

### From a release

Each release publishes an archive per platform, with the UI already embedded in
the binary, plus a `checksums.txt` and an SBOM.

```bash
# Pick the archive for your platform from the releases page, then
tar -xzf cubby_<version>_linux_amd64.tar.gz
./cubby version
./cubby start
```

The archive carries a `config.yaml` (a copy of `config.example.yaml`) alongside
the binary, which is what `start` reads by default.

### From source

```bash
# Use the config.yaml locally
cp config.example.yaml config.yaml

# Build the UI into server/pkg/modules/web/dist and compile the binaries into ./bin (linux/amd64 and windows/amd64)
make build

# Run it, then follow "First run" below
./bin/server.bin start
```

### From Docker Hub

Every release publishes a multi-arch image to [andreypostal/cubby](https://hub.docker.com/r/andreypostal/cubby):

```bash
docker pull andreypostal/cubby:{tag}
```

`{tag}` is `latest`, or a version pinned as far as you want: `1.2.3`, `1.2` or `1`.
The same image is mirrored to `ghcr.io/andreypostal/cubby`.

```bash
docker run -d --name cubby \
  -p 8080:8080 \
  -v cubby-data:/data \
  -v cubby-backups:/backups \
  andreypostal/cubby:latest

docker logs cubby | grep setup_token
```

### With Docker

Building the image locally, from this repository:

```bash
docker compose up -d --build
docker compose logs cubby | grep setup_token
```

### First run

On the first start the system will write a one-time setup token to its log:

```
WARN Setup required: this server has no users yet setup_token=7f3a9c1e... hint="open the web UI and enter this token to create the first administrator"
```

Open the UI and use the token to setup the first system admin user. Other users can be managed in the server settings page.

A deployment with no way to read the log, or one being scripted, can still create that first user directly using cli, which also ends the setup phase:

```bash
docker compose exec cubby server setup new-user -u admin -p 's3cret' -n 'Admin' --admin
```

## Configuration

Values are read from the YAML file first (`-config`, default `config.yaml`), then overridden by environment variables. The full set is in `server/pkg/core/config.go`.

## Development

### Tests

There are two tiers, and they cover different failures:

| Command            | What it runs                                                              |
| ------------------ | ------------------------------------------------------------------------- |
| `make test`        | Unit tests, package by package                                            |
| `make test-race`   | The same with the race detector and shuffled order, which is what CI runs |
| `make test-system` | Starts a real binary on a real port and drives it over HTTP               |
| `make test-all`    | Both tiers                                                                |
| `make ci`          | The full pre-push check: lint, race tests, system tests                   |

### Other checks

```bash
make lint          # go vet + eslint
make fmt           # gofmt + prettier
make vulncheck     # govulncheck against the module
make release-check # a full goreleaser snapshot, without tagging or publishing
make docker-build  # the container image
```

## License

MIT — see [LICENSE](LICENSE).
