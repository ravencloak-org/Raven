package marketplace

import (
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
)

// PublishedKBProjection is the typed shape of "what crosses the Marketplace
// publish boundary". ADR-0002 names this the content-grade fork projection:
// KB metadata, source rows (no file blobs), documents (no blob refs), chunks,
// and embeddings whose model matches the destination's required model.
//
// Hard-deny is structural, not runtime. This struct has NO fields for
// api_keys, chat_sessions, routing_rules, webhook_configs, response_cache,
// airbyte_connectors, or any per-row file-blob reference (documents.storage_path
// is intentionally omitted from ProjectedDocument). A future contributor who
// wants to leak those tables has to first add a field here, which forces a PR,
// a code review, and a deliberate ADR amendment — silent widening is
// impossible.
//
// The SQL function `marketplace_import_kb` (migration 00055) mirrors this
// struct field-for-field in its INSERT column lists. The test
// TestProjectionStructMatchesSQLFunction pins the two surfaces together so
// drift on either side fails the build.
type PublishedKBProjection struct {
	// KBMetadata is the projected subset of the source KB row.
	KBMetadata ProjectedKBMetadata
	// SourceLicenseSPDXID is the SPDX identifier carried over from the
	// source — the Marketplace publish CHECK constraint guarantees this is
	// non-empty for any source that survived the listing filter.
	SourceLicenseSPDXID string
	// ImportedFromRevisionAt is the source's last_modified_at at the
	// moment of import — the freshness pointer ADR-0003 uses to compute
	// "stale relative to source" without a Marketplace round-trip.
	ImportedFromRevisionAt time.Time
	// Sources is the source rows that cross the boundary. Metadata only —
	// no file blob references (sources never carry blobs to begin with,
	// but the SQL function's column list documents this explicitly).
	Sources []ProjectedSource
	// Documents is the document rows that cross the boundary. Note: no
	// `StoragePath` field — the SeaweedFS blob is publisher-private per
	// ADR-0002. ParseProjectedDocument-style helpers cannot reach for it
	// because the field does not exist on this struct.
	Documents []ProjectedDocument
	// Chunks carries the chunked text that powers retrieval in the
	// importer's RAG path. Re-keyed to the new KB at apply time inside
	// the SQL function.
	Chunks []ProjectedChunk
	// Embeddings carries the vector embeddings that match the destination
	// Org's required embedding model. If the projection is computed for a
	// destination whose model does not match the source's, this slice is
	// empty and the Go layer must raise ErrEmbeddingModelMismatch BEFORE
	// any write (re-embed flow deferred per ADR-0001).
	Embeddings []ProjectedEmbedding
}

// ProjectedKBMetadata is the subset of model.KnowledgeBase fields that ADR-0002
// names as boundary-crossing. Notably absent: api keys, chat sessions, routing
// rules, webhook configs, response cache, airbyte connectors (none of which
// live on knowledge_bases in any case, but the principle holds for the
// settings JSONB scrub below).
type ProjectedKBMetadata struct {
	Name        string
	Slug        string
	Description string
	// Settings is the scrubbed JSONB payload. ProjectedKBMetadata.Settings
	// is the OUTPUT of `scrubSettings` — it is already allow-list-filtered.
	// Callers must never put raw source settings here.
	Settings map[string]any
}

// ProjectedSource is the metadata-only projection of a sources row. No file
// blob fields because the source rows never carry blobs (they are URL /
// sitemap / RSS pointers); the explicit field list documents which columns
// cross the boundary and which do not (processing_status / processing_error
// are reset, not copied — the import is "ready" by construction).
type ProjectedSource struct {
	SourceType     model.SourceType
	URL            string
	CrawlDepth     int
	CrawlFrequency model.CrawlFrequency
	Title          string
	PagesCrawled   int
	Metadata       map[string]any
}

// ProjectedDocument is the metadata-only projection of a documents row.
//
// Hard-deny: there is NO StoragePath field on this struct. SeaweedFS blob
// references are publisher-private per ADR-0002 — the importer's UI cannot
// download the original PDF/Word/etc, only the chunks the publisher's
// chunker emitted. A future contributor who wants to flip this needs to
// add the field here AND in the SQL function's INSERT list AND change
// ADR-0002 — three deliberate steps.
type ProjectedDocument struct {
	FileName      string
	FileType      string
	FileSizeBytes *int64
	FileHash      string
	Title         string
	PageCount     *int
	Metadata      map[string]any
}

// ProjectedChunk is the text-only projection of a chunks row. The (Content,
// ChunkIndex) pair is the identity used to re-link embeddings to the new
// chunks inside the SQL apply step.
type ProjectedChunk struct {
	SourceFileHash *string
	SourceURL      *string
	Content        string
	ChunkIndex     int
	TokenCount     *int
	PageNumber     *int
	Heading        *string
	ChunkType      model.ChunkType
	Metadata       map[string]any
}

// ProjectedEmbedding is the vector-only projection of an embeddings row.
// Only present in the projection when the source's `model_name` matches the
// destination Org's required embedding model — otherwise the Go layer fails
// loud with ErrEmbeddingModelMismatch BEFORE any write happens.
type ProjectedEmbedding struct {
	ChunkContent string
	ChunkIndex   int
	Vector       []float32
	ModelName    string
	ModelVersion *string
	Dimensions   int
}

// SourceKB is the input to projectPublished — the raw source-side state that
// the Go layer reads through a SECURITY DEFINER bridge before applying the
// projection. The struct mirrors what marketplace_import_kb reads from
// `knowledge_bases` plus the child collections; in production the Go layer
// does not actually pass SourceKB to the SQL function (the SQL function
// reads the source directly), but the test path uses projectPublished
// directly to assert the boundary on a fixture.
type SourceKB struct {
	KB         *model.KnowledgeBase
	Sources    []model.Source
	Documents  []model.Document
	Chunks     []model.Chunk
	Embeddings []model.Embedding
}

// projectPublished is the canonical "what crosses the publish boundary"
// function. ONE function, taking the typed source state and returning the
// typed projection. No 12-helper fan-out — every projection rule lives here
// so review and ADR amendment have one file to track.
//
// Free Plan visibility override (ADR-0004) lives ABOVE this function in
// Importer.Import — projectPublished is pure and does not know whether the
// destination Org is on a Free Plan. Keeping the projection oblivious is
// what makes the unit tests for it trivial (one input, one output) and what
// makes the Free Plan rule legible from a single line in Importer.Import.
//
// requiredEmbeddingModel selects which embeddings cross. Empty string means
// "destination has no default; do not copy embeddings" (the importer can
// re-embed locally — that work is deferred per ADR-0001 §Trade-offs and
// surfaces as ErrEmbeddingModelMismatch from Importer.Import when the source
// has embeddings of a different model). projectPublished itself does NOT
// raise on mismatch — it simply omits non-matching embeddings; the loud
// failure is the orchestrator's call.
func projectPublished(src SourceKB, requiredEmbeddingModel string) PublishedKBProjection {
	srcKB := src.KB

	out := PublishedKBProjection{
		KBMetadata: ProjectedKBMetadata{
			Name:        srcKB.Name,
			Slug:        srcKB.Slug,
			Description: srcKB.Description,
			Settings:    scrubSettings(srcKB.Settings),
		},
	}

	if srcKB.LicenseSPDXID != nil {
		out.SourceLicenseSPDXID = *srcKB.LicenseSPDXID
	}
	out.ImportedFromRevisionAt = srcKB.LastModifiedAt

	// Sources: metadata only. processing_status / processing_error reset
	// at apply time inside the SQL function — see migration 00055 §9.
	out.Sources = make([]ProjectedSource, 0, len(src.Sources))
	for _, s := range src.Sources {
		out.Sources = append(out.Sources, ProjectedSource{
			SourceType:     s.SourceType,
			URL:            s.URL,
			CrawlDepth:     s.CrawlDepth,
			CrawlFrequency: s.CrawlFrequency,
			Title:          s.Title,
			PagesCrawled:   s.PagesCrawled,
			Metadata:       s.Metadata,
		})
	}

	// Documents: metadata only. NO StoragePath — hard-deny via missing
	// field on ProjectedDocument (see struct comment).
	out.Documents = make([]ProjectedDocument, 0, len(src.Documents))
	for _, d := range src.Documents {
		out.Documents = append(out.Documents, ProjectedDocument{
			FileName:      d.FileName,
			FileType:      d.FileType,
			FileSizeBytes: d.FileSizeBytes,
			FileHash:      d.FileHash,
			Title:         d.Title,
			PageCount:     d.PageCount,
			Metadata:      d.Metadata,
		})
	}

	// Build a fast (chunk_id → file_hash / source_url) lookup so the
	// chunk projection records the parent identity instead of a stale
	// FK. The SQL function re-resolves these to new ids at apply time.
	docHashByID := make(map[string]string, len(src.Documents))
	for _, d := range src.Documents {
		docHashByID[d.ID] = d.FileHash
	}
	srcURLByID := make(map[string]string, len(src.Sources))
	for _, s := range src.Sources {
		srcURLByID[s.ID] = s.URL
	}

	out.Chunks = make([]ProjectedChunk, 0, len(src.Chunks))
	for _, c := range src.Chunks {
		var docHash, srcURL *string
		if c.DocumentID != nil {
			if h, ok := docHashByID[*c.DocumentID]; ok && h != "" {
				h := h
				docHash = &h
			}
		}
		if c.SourceID != nil {
			if u, ok := srcURLByID[*c.SourceID]; ok && u != "" {
				u := u
				srcURL = &u
			}
		}
		out.Chunks = append(out.Chunks, ProjectedChunk{
			SourceFileHash: docHash,
			SourceURL:      srcURL,
			Content:        c.Content,
			ChunkIndex:     c.ChunkIndex,
			TokenCount:     c.TokenCount,
			PageNumber:     c.PageNumber,
			Heading:        c.Heading,
			ChunkType:      c.ChunkType,
			Metadata:       c.Metadata,
		})
	}

	// Embeddings — pre-filter to the destination's required model. The
	// orchestrator (Importer.Import) checks for the "source has
	// embeddings but none match" failure mode and raises
	// ErrEmbeddingModelMismatch BEFORE writing. projectPublished is
	// silent because the decision to fail-loud is policy, not
	// projection.
	chunkContentByID := make(map[string]string, len(src.Chunks))
	chunkIdxByID := make(map[string]int, len(src.Chunks))
	for _, c := range src.Chunks {
		chunkContentByID[c.ID] = c.Content
		chunkIdxByID[c.ID] = c.ChunkIndex
	}
	out.Embeddings = make([]ProjectedEmbedding, 0, len(src.Embeddings))
	for _, e := range src.Embeddings {
		if requiredEmbeddingModel != "" && e.ModelName != requiredEmbeddingModel {
			continue
		}
		out.Embeddings = append(out.Embeddings, ProjectedEmbedding{
			ChunkContent: chunkContentByID[e.ChunkID],
			ChunkIndex:   chunkIdxByID[e.ChunkID],
			Vector:       e.Embedding.Slice(),
			ModelName:    e.ModelName,
			ModelVersion: e.ModelVersion,
			Dimensions:   e.Dimensions,
		})
	}

	return out
}

// scrubSettings is the JSONB allow-list filter. It returns a NEW map; the
// input is never mutated. Default deny: only keys explicitly named in
// settingsAllowList (or prefixed `public:`) survive.
//
// Implementation is a single linear pass — the allow-list is small (a
// handful of keys), and a more clever set lookup would obfuscate the rule
// at no measurable benefit.
func scrubSettings(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isAllowedSettingsKey(k) {
			out[k] = v
		}
	}
	return out
}
