Review of the CineSeek Go backend structure

Suggested structure

```text
cineseek-backend/
├── cmd/
│   ├── api/main.go                  # HTTP server only (thin: load config → build app → run)
│   └── worker/main.go              # Background indexer/refresher (megathread sync, link health checks)
├── internal/
│   ├── app/
│   │   ├── app.go                  # Dependency wiring (composition root), graceful shutdown
│   │   └── wire.go                 # Optional: google/wire
│   ├── config/
│   │   └── config.go               # Typed config + validation at startup (fail fast)
│   ├── domain/                     # Entities + errors only (no interfaces here)
│   │   ├── media.go
│   │   ├── stream.go
│   │   └── errors.go               # ErrNotFound, ErrProviderTimeout, ...
│   ├── usecase/                    # Interfaces (ports) live HERE, next to consumer
│   │   ├── ports.go                # Cache, MetadataClient, Provider, IndexStore
│   │   ├── search.go
│   │   ├── streams.go              # Get streams for a media ID
│   │   ├── indexer.go              # Megathread indexing (called by worker, not request path)
│   │   └── search_test.go
│   ├── transport/
│   │   └── http/
│   │       ├── router.go
│   │       ├── handler_media.go
│   │       ├── handler_health.go   # /healthz, /readyz
│   │       ├── dto.go              # Request/response structs (don't leak domain types)
│   │       ├── errors.go           # domain error → HTTP status mapping
│   │       └── middleware/
│   │           ├── requestid.go
│   │           ├── logging.go
│   │           ├── ratelimit.go
│   │           ├── recover.go
│   │           └── cors.go
│   ├── infra/
│   │   ├── cache/
│   │   │   ├── redis.go
│   │   │   └── memory.go           # In-proc L1 cache (ristretto/otter) + tests
│   │   ├── store/
│   │   │   └── postgres/           # Durable index (megathread results, link health)
│   │   │       ├── migrations/
│   │   │       └── index_repo.go
│   │   ├── metadata/
│   │   │   └── tmdb.go             # Canonical IDs (tmdb_id / imdb_id) & posters
│   │   └── httpclient/
│   │       └── client.go           # Shared client: timeouts, retries+jitter, UA, proxy support
│   └── scraper/
│       ├── registry.go             # Register providers; enable/disable via config
│       ├── pool.go                 # Fan-out/fan-in (errgroup + semaphore)
│       ├── resilience.go           # Per-provider rate limiter + circuit breaker
│       ├── normalize.go            # Title normalization, year/quality/lang parsing
│       ├── megathread/
│       │   └── reddit.go
│       └── providers/
│           ├── provider_a/
│           │   ├── provider.go     # Fetch
│           │   ├── parser.go       # Pure parse(html) → []StreamLink (easy to test)
│           │   └── testdata/       # Saved HTML fixtures
│           └── provider_b/
├── api/
│   └── openapi.yaml                # API contract
├── deployments/
│   ├── Dockerfile                  # Multi-stage, distroless, non-root
│   └── docker-compose.yml          # api + worker + redis + postgres
├── .env.example
├── .golangci.yml
├── Makefile
├── go.mod
└── go.sum
```

	

s and why

1. Define interfaces where they're consumed, not in domain.
   Idiomatic Go puts small interfaces next to the code that uses them. Keeping domain to entities and errors avoids a grab-bag interfaces.go that grows forever and forces mocks of huge interfaces.
2. Take megathread indexing off the request path.
   This is the biggest architectural issue. A user search shouldn't trigger Reddit parsing. Add cmd/worker that periodically builds the index into durable storage, and have the API only read it. Redis alone is a poor home for this: an eviction or restart loses your index. Use Postgres (or Redis with persistence and explicit key design) as the source of truth, with Redis as a cache.
3. Key everything on canonical IDs, not titles.
   Add a TMDB/IMDb metadata client. Scraping by title string produces mismatches ("Dune" vs "Dune: Part Two"). Resolve the query to a tmdb_id first, then match providers against ID + year + normalized title. This also gives you posters, ratings, and better autocomplete for free.
4. Give scrapers a strict contract and separate fetch from parse.

go
// usecase/ports.go
type Provider interface {
    Name() string
    Search(ctx context.Context, q domain.MediaQuery) ([]domain.StreamLink, error)
}

Each provider splits into provider.go (network) and parser.go (pure function on HTML). You can then unit-test parsers against saved fixtures in testdata/, which matters because scrapers break constantly when sites change markup.

5. Use errgroup with a limit and per-provider timeouts.

go
func (p *Pool) Search(ctx context.Context, q domain.MediaQuery) ([]domain.StreamLink, []error) {
    g, ctx := errgroup.WithContext(ctx) // or plain WaitGroup if you want partial results
    g.SetLimit(p.maxConcurrent)
    // each provider: context.WithTimeout(ctx, 4*time.Second), collect results via channel
}

A slow provider shouldn't block the response. Return partial results and record which providers failed. Consider streaming results to the client over SSE as each provider finishes; it feels much faster.

6. Prevent cache stampedes.
   Wrap cache-miss scraping with golang.org/x/sync/singleflight so 100 concurrent searches for the same title trigger one scrape. Add stale-while-revalidate (serve stale, refresh in the background) and short negative caching for "no results".
7. Add resilience per provider. A rate limiter (x/time/rate), circuit breaker (sony/gobreaker), and retry with exponential backoff and jitter live in resilience.go. When provider B is down, the breaker opens and you stop hammering it.
8. Link health checking. Dead links are the main quality problem for this kind of product. The worker should HEAD-check stored links, score them, and drop or demote broken ones. Also validate outbound URLs (scheme allowlist, no internal IPs) so scraped data can't turn into an SSRF vector.
9. Rename and simplify a few things.

handler/http → transport/http (clearer, and add a dto.go so you don't serialize domain entities directly).
repository/redis → infra/cache, since a cache isn't really a repository.
Drop pkg/logger. Go 1.21+ ships log/slog, so configure it in app instead of maintaining a wrapper. Anything in pkg/ is a public API promise, so keep utilities in internal/ unless you truly intend to share them.
Move workerpool.go alongside the scraper code as pool.go. It's not generic enough to justify a public package.

10. Operational essentials.

API: version routes (/api/v1/...), add pagination, /healthz and /readyz, request IDs, and proper context cancellation.
Observability: Prometheus metrics per provider (latency, error rate, results count), plus OpenTelemetry traces. The per-provider metrics tell you immediately which scraper broke.
Shutdown: signal.NotifyContext plus http.Server.Shutdown so in-flight scrapes finish cleanly.
Config: validate on boot and fail fast; provide .env.example.
Tooling: Makefile, golangci-lint, CI running go test -race ./..., and integration tests with testcontainers-go for Redis/Postgres.
Suggested build order
TMDB client + canonical ID resolution
Provider interface + one provider with fixture-based parser tests
Pool with timeouts, singleflight, and cache layers
Worker + durable index for megathreads
Resilience (breaker/limiter), metrics, link health checks
