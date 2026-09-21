# Testing & Verification Guide

This document outlines how to execute, manage, and troubleshoot tests for the `cineseek-backend` microservice.

---

## 1. Environment & Prerequisites

Ensure Go modules are properly initialized at the project root before running test suites:

```bash
# Verify Go module dependencies
go mod tidy
```

### Module Structure

* **Module Root:** `cineseek-backend`
* **Domain Model:** `cineseek-backend/internal/domain`
* **Scraper Implementations:** `cineseek-backend/internal/scraper/megathread`

---

## 2. Running Test Suites

### A. Fast / Offline Unit Tests (Recommended for CI/CD)

To run local unit tests without hitting external networks or triggering rate limits:

```bash
go test -v -short ./...
```

* **`-short` flag:** Instructs integration tests (like `TestLiveRedditWikiFetch`) to skip live network calls, ensuring fast, offline test execution.

---

### B. Live Integration Tests

To test live scraper endpoints against actual web sources:

```bash
go test -v ./internal/scraper/megathread/...
```

* **`-race` flag (Optional):** Detects race conditions during concurrent test executions:
  ```bash
  go test -v -race ./internal/scraper/megathread/...
  ```
