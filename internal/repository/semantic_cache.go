package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
)

// CacheEntry is a single row of the response_cache table, scoped to an org.
// Only the fields required by callers are materialised here.
type CacheEntry struct {
	ID          string
	Query       string
	Answer      string
	Similarity  float32 // only populated by LookupSimilar
	HitCount    int
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CacheStats is the response shape for GET /cache/stats.
type CacheStats struct {
	TotalEntries         int64     `json:"total_entries"`
	TotalHits            int64     `json:"hit_count"`
	EstimatedTokensSaved int64     `json:"estimated_tokens_saved"`
	ExpiresSoonest       *time.Time `json:"expires_soonest,omitempty"`
	AvgHits              float64   `json:"avg_hits"`
}

// estimatedTokensPerHit is a rough per-cache-hit saved-token estimate used
// when the row doesn't carry an explicit token count. Derived from internal
// telemetry: typical RAG answer ~ 450 tokens + ~800 context tokens.
const estimatedTokensPerHit = 1250

// SemanticCacheRepository manages the response_cache table.
type SemanticCacheRepository struct {
	pool *pgxpool.Pool
}

// NewSemanticCacheRepository creates a new SemanticCacheRepository.
func NewSemanticCacheRepository(pool *pgxpool.Pool) *SemanticCacheRepository {
	return &SemanticCacheRepository{pool: pool}
}

// InvalidateKB deletes all cache entries for a knowledge base. Returns the
// number of rows removed. Safe to call fire-and-forget from document-edit
// paths — RLS is enforced via db.WithOrgID.
func (r *SemanticCacheRepository) InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error) {
	var rowsAffected int64
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM response_cache WHERE org_id = $1::uuid AND kb_id = $2::uuid`,
			orgID, kbID,
		)
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		rowsAffected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("SemanticCacheRepository.InvalidateKB: %w", err)
	}
	return rowsAffected, nil
}

// Stats returns aggregate cache metrics for a single KB. Expired rows are
// excluded from every aggregate so the numbers mirror what a live lookup
// would actually hit.
//
// Issue #256 — acceptance: the response must carry total_entries, hit_count,
// estimated_tokens_saved and expires_soonest.
func (r *SemanticCacheRepository) Stats(ctx context.Context, orgID, kbID string) (CacheStats, error) {
	var s CacheStats
	var earliestExpiry *time.Time
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT
			   COUNT(*),
			   COALESCE(SUM(hit_count), 0),
			   COALESCE(AVG(hit_count), 0),
			   MIN(expires_at)
			 FROM response_cache
			 WHERE org_id = $1::uuid AND kb_id = $2::uuid AND expires_at > NOW()`,
			orgID, kbID,
		)
		if scanErr := row.Scan(&s.TotalEntries, &s.TotalHits, &s.AvgHits, &earliestExpiry); scanErr != nil {
			return fmt.Errorf("scan: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return CacheStats{}, fmt.Errorf("SemanticCacheRepository.Stats: %w", err)
	}
	s.ExpiresSoonest = earliestExpiry
	s.EstimatedTokensSaved = s.TotalHits * estimatedTokensPerHit
	return s, nil
}

// bumpHitTimeout bounds how long a detached hit_count UPDATE may run after
// LookupSimilar has already returned its result. Kept small — the work is a
// single UPDATE on a row we just selected by id.
const bumpHitTimeout = 5 * time.Second

// LookupSimilar performs a cosine-similarity search against the HNSW index
// and returns at most one hit whose similarity is at least threshold.
//
// NOTE: The production hot path for semantic cache lookup runs in the Python
// AI worker (see ai-worker/raven_worker/retrieval/cache_repo.py) because the
// embedding is produced there and shipping raw vectors across RPC boundaries
// would double p99 latency. This Go method exists for operator tooling and
// for the benchmark that enforces the ≤ 10 ms p99 acceptance criterion.
//
// The hit_count UPDATE is dispatched in a detached goroutine after the SELECT
// transaction commits, so callers never pay the round-trip. This matches the
// Python implementation's fire-and-forget contract.
func (r *SemanticCacheRepository) LookupSimilar(
	ctx context.Context,
	orgID, kbID string,
	embedding []float32,
	threshold float32,
) (*CacheEntry, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("SemanticCacheRepository.LookupSimilar: empty embedding")
	}
	vec := vectorLiteral(embedding)

	var entry *CacheEntry
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT id::text, query_text, response_text,
			        1 - (query_embedding <=> $1::vector) AS similarity,
			        hit_count, created_at, expires_at
			 FROM response_cache
			 WHERE org_id = $2::uuid
			   AND kb_id  = $3::uuid
			   AND expires_at > NOW()
			   AND 1 - (query_embedding <=> $1::vector) >= $4
			 ORDER BY query_embedding <=> $1::vector
			 LIMIT 1`,
			vec, orgID, kbID, threshold,
		)
		var e CacheEntry
		scanErr := row.Scan(&e.ID, &e.Query, &e.Answer, &e.Similarity, &e.HitCount, &e.CreatedAt, &e.ExpiresAt)
		if scanErr != nil {
			if scanErr == pgx.ErrNoRows {
				return nil
			}
			return fmt.Errorf("scan: %w", scanErr)
		}
		entry = &e
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("SemanticCacheRepository.LookupSimilar: %w", err)
	}
	if entry != nil {
		go r.bumpHitCount(orgID, entry.ID)
	}
	return entry, nil
}

// bumpHitCount runs the hit_count UPDATE on its own short-lived context and
// connection. Called as a goroutine from LookupSimilar so the caller is not
// blocked on UPDATE + commit round-trips. Errors are swallowed — a missed
// bump only affects Stats accuracy, never correctness.
func (r *SemanticCacheRepository) bumpHitCount(orgID, rowID string) {
	ctx, cancel := context.WithTimeout(context.Background(), bumpHitTimeout)
	defer cancel()
	_ = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`UPDATE response_cache SET hit_count = hit_count + 1 WHERE id = $1`,
			rowID,
		)
		return err
	})
}

// Store writes a new cache entry. Called after a successful LLM generation
// on a cache miss. Expiry is set by the column default (7 days after
// migration 00036).
func (r *SemanticCacheRepository) Store(
	ctx context.Context,
	orgID, kbID, queryText, answerText string,
	embedding []float32,
	metadata map[string]any,
) (string, error) {
	if len(embedding) == 0 {
		return "", fmt.Errorf("SemanticCacheRepository.Store: empty embedding")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	vec := vectorLiteral(embedding)

	var id string
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`INSERT INTO response_cache (org_id, kb_id, query_text, query_embedding, response_text, metadata)
			 VALUES ($1::uuid, $2::uuid, $3, $4::vector, $5, $6::jsonb)
			 RETURNING id::text`,
			orgID, kbID, queryText, vec, answerText, metadata,
		)
		return row.Scan(&id)
	})
	if err != nil {
		return "", fmt.Errorf("SemanticCacheRepository.Store: %w", err)
	}
	return id, nil
}

// vectorLiteral converts a []float32 embedding to the pgvector string
// representation ("[0.12,0.34,...]") so it can be cast to vector inside SQL.
func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	// Pre-size the buffer so we don't re-allocate for 1536-dim vectors.
	buf := make([]byte, 0, 16*len(v)+2)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = appendFloat32(buf, f)
	}
	buf = append(buf, ']')
	return string(buf)
}

// appendFloat32 formats f with enough precision to round-trip through pgvector.
func appendFloat32(dst []byte, f float32) []byte {
	return fmt.Appendf(dst, "%.7g", f)
}
