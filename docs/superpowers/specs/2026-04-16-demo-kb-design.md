# Demo Knowledge Base — Design Spec

**Date**: 2026-04-16
**Milestone**: M10: Demo Experience
**Status**: Approved

## Goal

Create a shared demo organisation with a TMDB-powered knowledge base that:
1. Gives new users an immediate "try it" experience on signup
2. Doubles as a developer/CI fixture that exercises the full ingest pipeline

## Data Source

**TMDB (The Movie Database)** — top-rated movies across 5 genres (Action, Comedy, Drama, Sci-Fi, Animation), 10 per genre = 50 movies for the `small` size.

Each movie is fetched with `append_to_response=credits,reviews,keywords` and rendered as a markdown document:

```markdown
# The Godfather (1972)

## Overview
The aging patriarch of an organized crime dynasty...

## Details
- **Genres:** Crime, Drama
- **Director:** Francis Ford Coppola
- **Cast:** Marlon Brando, Al Pacino, James Caan...
- **Rating:** 8.7/10 (19,234 votes)
- **Runtime:** 175 min
- **Keywords:** mafia, crime family, godfather...

## Reviews
> "A masterpiece of American cinema..." — user123
```

### Size Tiers

| Size | Movies | Seed Time | Use Case |
|------|--------|-----------|----------|
| `small` | 50 | ~30-60s | CI, dev, initial release |
| `medium` | 500 | ~5-10 min | Production (future) |
| `large` | 2000 | ~20-30 min | Scale testing (future) |

Only `small` is implemented initially. `medium`/`large` return `501 Not Implemented`.

## Architecture

### API Endpoint

```
POST /api/v1/admin/seed-demo?size=small
```

**Auth:** Session middleware + `admin` role check. Also accepts `X-Seed-Key` header matched against `RAVEN_SEED_KEY` env var for Docker entrypoint / CI invocation before any user exists.

**Idempotency:** Looks up org by slug `demo`. If exists, returns current state without re-seeding.

**Response:**
```json
{
  "org_id": "uuid",
  "workspace_id": "uuid",
  "kb_id": "uuid",
  "documents_enqueued": 50,
  "pipeline_status": "seeding"
}
```

### Seed Pipeline Sequence

1. Check idempotency — `SELECT` org by slug `demo`. Return early if exists.
2. Create demo org — `OrgService.Create()` with name "Raven Demo", slug `demo`.
3. Create workspace — `WorkspaceService.Create()` with name "Movies", slug `movies`.
4. Create knowledge base — `KBService.Create()` with name "Movie Database", slug `movie-database`.
5. Fetch TMDB data — `tmdb.Client.FetchTopByGenres(ctx, size)`.
6. Create documents — render markdown, upload to SeaweedFS, create document records with status `queued`.
7. Enqueue pipeline jobs — `task:process_document` Asynq jobs per document (existing worker handles chunk + embed).
8. Return response with `pipeline_status: "seeding"`.

### User Auto-Join

In the SuperTokens signup callback (or `UserLookup` middleware on first login):
- Check if demo org exists (by slug `demo`)
- If user is not already a member, add as `viewer` role on the demo workspace
- Single INSERT, no new endpoint needed

### TMDB Client

**Package:** `internal/tmdb`

**Config:** `TMDB_API_KEY` env var, added to `Config` as `TMDBConfig`.

**Fetch strategy:**
- `GET /discover/movie?sort_by=vote_average.desc&with_genres={id}&vote_count.gte=1000` — 10 per genre
- `GET /movie/{id}?append_to_response=credits,reviews,keywords` — full details per movie
- Semaphore-limited to 40 req/s (TMDB rate limit)

## File Structure

### New Files

| File | Purpose |
|------|---------|
| `internal/tmdb/client.go` | TMDB API client |
| `internal/tmdb/models.go` | Response structs |
| `internal/tmdb/markdown.go` | Movie → markdown renderer |
| `internal/tmdb/client_test.go` | Unit tests with mock HTTP server |
| `internal/tmdb/markdown_test.go` | Markdown rendering tests |
| `internal/handler/seed.go` | `POST /api/v1/admin/seed-demo` handler |
| `internal/handler/seed_test.go` | Handler tests |

### Modified Files

| File | Change |
|------|--------|
| `cmd/api/main.go` | Register seed endpoint, wire TMDB client |
| `internal/config/config.go` | Add `TMDBConfig` struct + env binding |
| `internal/middleware/auth.go` | Add `X-Seed-Key` bypass for seed endpoint |
| Signup callback | Auto-join demo org as viewer |

## Testing

- **`internal/tmdb/`** — unit tests with `httptest.Server` mocking TMDB responses. No real API calls.
- **`internal/handler/seed_test.go`** — mock service interfaces, test idempotency, missing API key error, auth bypass.
- **CI integration test** — seed endpoint with mock TMDB, verify documents created and Asynq jobs enqueued.

## Out of Scope (Future)

- `medium`/`large` size tiers
- Demo KB auto-refresh (re-seed with fresh TMDB data on schedule)
- Demo KB in frontend onboarding wizard UI
- Pre-computed embeddings shipped as fixtures
