# Agent Guidelines for CineProbe Backend

## Project Overview
**CineProbe** is an automated web aggregator and search engine that parses Reddit megathreads to locate and display streaming sources for movies and TV shows.

This repository houses the **Go Backend**, designed following Clean Architecture and Domain-Driven Design principles. It features an API layer for serving client queries, a background worker for asynchronous index refreshing/link health checks, and a resilient scraping engine.

## Core Architecture & Design Principles
1. **Ports & Adapters (Clean Architecture):**
   - Interfaces live alongside their consumers in `internal/usecase/ports.go`. Do NOT define interfaces in `internal/domain/`.
   - `internal/domain/` strictly contains pure domain entities (`media.go`, `stream.go`) and error definitions (`errors.go`).
2. **Canonical IDs First:** Always resolve search queries via TMDB (`internal/infra/metadata/tmdb.go`) to acquire a `tmdb_id` or `imdb_id` before querying scrapers or caches. Never rely solely on loose title strings.
3. **Off-Request Path Indexing:** Megathread parsing and heavy indexing tasks belong in `cmd/worker/`, not in the HTTP request/response lifecycle.
4. **Scraper Separation:** Every scraper provider under `internal/scraper/providers/` must strictly separate HTTP execution (`provider.go`) from HTML parsing logic (`parser.go`). Pure parse functions must be tested via static HTML fixtures in a local `testdata/` directory.
5. **Resilience & Concurrency:** Use `errgroup` with concurrency limits, per-provider timeouts, circuit breakers (`gobreaker`), and singleflight (`golang.org/x/sync/singleflight`) to avoid cache stampedes.

---

## Repository Structure

```text
cineprobe-backend/
├── cmd/
│   ├── api/main.go                  # HTTP server entry point
│   └── worker/main.go               # Background indexer & link health checker
├── internal/
│   ├── app/
│   │   ├── app.go                  # Dependency wiring (composition root) & graceful shutdown
│   │   └── wire.go                 # Google Wire dependency injection setup (optional)
│   ├── config/
│   │   └── config.go               # Environment configuration with fail-fast validation
│   ├── domain/                     # Entities and domain-specific errors ONLY
│   │   ├── media.go                # Media structures
│   │   ├── stream.go               # StreamLink structures
│   │   └── errors.go               # Sentinel errors (ErrNotFound, ErrProviderTimeout)
│   ├── usecase/                    # Application logic and input/output ports
│   │   ├── ports.go                # Store, Cache, Metadata, Provider interfaces
│   │   ├── search.go               # Core media search use case
│   │   ├── streams.go              # Stream fetching logic
│   │   └── indexer.go              # Megathread indexing use case (used by worker)
│   ├── transport/
│   │   └── http/                   # HTTP delivery layer
│   │       ├── router.go           # Route definitions
│   │       ├── handler_media.go    # Media & Stream endpoints
│   │       ├── handler_health.go   # /healthz and /readyz endpoints
│   │       ├── dto.go              # Request & Response Data Transfer Objects
│   │       ├── errors.go           # Mapping domain errors to HTTP statuses
│   │       └── middleware/         # Request ID, Logging (slog), Rate Limiting, Recovery, CORS
│   ├── infra/                      # External infrastructure adapters
│   │   ├── cache/                  # Redis & in-memory L1 cache (Otter/Ristretto)
│   │   ├── store/postgres/         # Persistence layer for indexed megathreads & link health
│   │   │   ├── migrations/         # SQL migration files
│   │   │   └── index_repo.go       # Index repository implementation
│   │   ├── metadata/               # TMDB / IMDb API integration
│   │   └── httpclient/             # Shared HTTP client (timeouts, retries, user-agent, proxies)
│   └── scraper/                    # Scraping engine
│       ├── registry.go             # Provider registry
│       ├── pool.go                 # Fan-out/fan-in scraper engine (errgroup + semaphore)
│       ├── resilience.go           # Rate limiters & circuit breakers
│       ├── normalize.go            # Title normalization, quality & language parsing
│       ├── megathread/             # Reddit megathread parser
│       └── providers/              # Specific streaming source scrapers
│           └── provider_a/
│               ├── provider.go     # Network execution
│               ├── parser.go       # Pure HTML/JSON parser
│               └── testdata/       # HTML fixtures for tests
├── api/
│   └── openapi.yaml                # OpenAPI specification contract
├── deployments/
│   ├── Dockerfile                  # Multi-stage, distroless, non-root build
│   └── docker-compose.yml          # Local stack (API, Worker, Redis, Postgres)
├── .env.example
├── .golangci.yml
├── Makefile
├── go.mod
└── go.sum
```

---

## Technical Conventions & Guidelines

### 1. Logging & Observability
- Use Go's standard `log/slog` for structured logging. Do not introduce custom logger wrappers in `pkg/`.
- Attach correlation IDs to logs using the `requestid` middleware context.

### 2. Error Handling & DTOs
- Do NOT expose domain entities directly to HTTP clients. Always map to/from DTOs defined in `internal/transport/http/dto.go`.
- Map sentinel domain errors (`domain.ErrNotFound`) to HTTP error responses centrally in `internal/transport/http/errors.go`.

### 3. Safety & Resilience
- **SSRF Prevention:** Validate and sanitize all scraped outbound links (scheme allowlists, prohibit internal/private IP addresses).
- **Graceful Shutdown:** All long-running servers and background workers must listen to `os.Interrupt` and `SIGTERM` using `signal.NotifyContext` and execute `http.Server.Shutdown`.

---

## Build & Operational Commands

| Command | Action |
| :--- | :--- |
| `make run-api` | Runs the HTTP API service locally (`go run ./cmd/api`) |
| `make run-worker` | Runs the background indexer service (`go run ./cmd/worker`) |
| `make test` | Executes unit tests with race detection (`go test -v -race ./...`) |
| `make lint` | Runs `golangci-lint` check against the repository |
| `make docker-up` | Starts local stack via Docker Compose (Postgres, Redis, API, Worker) |

---

## Implementation Roadmap (Build Order)
When contributing or extending features, follow this sequence:
1. **Canonical Metadata:** TMDB client & canonical ID resolution (`internal/infra/metadata/`).
2. **Scraper Contract:** Provider interfaces + single provider with fixture-backed tests in `testdata/`.
3. **Concurrency Engine:** Scraper pool with timeouts, `singleflight`, and Redis/Memory cache layers.
4. **Worker & Storage:** Postgres durable indexer for Megathread sync.
5. **Resilience & Health Checks:** Circuit breakers, rate limiters, link HEAD-checks, and metrics.