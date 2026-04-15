//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

func TestIngestion(t *testing.T) {
	ctx := context.Background()

	t.Run("document_status_lifecycle", func(t *testing.T) {
		t.Run("happy_path_queued_to_ready", func(t *testing.T) {
			org := seedOrg(t, ctx, "lifecycle-happy-"+uuid.NewString()[:8])
			t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

			docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "happy.md", "queued")

			// Walk the happy path: queued -> crawling -> parsing -> chunking -> embedding -> ready
			transitions := []model.ProcessingStatus{
				model.ProcessingStatusCrawling,
				model.ProcessingStatusParsing,
				model.ProcessingStatusChunking,
				model.ProcessingStatusEmbedding,
				model.ProcessingStatusReady,
			}
			for _, next := range transitions {
				err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, next, "")
				require.NoError(t, err, "transition to %s should succeed", next)
			}

			// Verify final status
			doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
			require.NoError(t, err)
			require.Equal(t, model.ProcessingStatusReady, doc.ProcessingStatus)
		})

		t.Run("failure_from_each_intermediate_state", func(t *testing.T) {
			intermediateStates := []struct {
				name string
				path []model.ProcessingStatus // transitions to reach the intermediate state from queued
			}{
				{"queued_to_failed", nil},
				{"crawling_to_failed", []model.ProcessingStatus{model.ProcessingStatusCrawling}},
				{"parsing_to_failed", []model.ProcessingStatus{model.ProcessingStatusCrawling, model.ProcessingStatusParsing}},
				{"chunking_to_failed", []model.ProcessingStatus{model.ProcessingStatusCrawling, model.ProcessingStatusParsing, model.ProcessingStatusChunking}},
				{"embedding_to_failed", []model.ProcessingStatus{model.ProcessingStatusCrawling, model.ProcessingStatusParsing, model.ProcessingStatusChunking, model.ProcessingStatusEmbedding}},
			}

			for _, tc := range intermediateStates {
				t.Run(tc.name, func(t *testing.T) {
					org := seedOrg(t, ctx, "fail-"+uuid.NewString()[:8])
					t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

					docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "fail.md", "queued")

					// Walk to the intermediate state
					for _, step := range tc.path {
						err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, step, "")
						require.NoError(t, err)
					}

					// Transition to failed
					err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusFailed, "simulated error")
					require.NoError(t, err)

					doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
					require.NoError(t, err)
					require.Equal(t, model.ProcessingStatusFailed, doc.ProcessingStatus)
				})
			}
		})

		t.Run("recovery_failed_to_queued", func(t *testing.T) {
			org := seedOrg(t, ctx, "recover-q-"+uuid.NewString()[:8])
			t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

			docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "recover.md", "queued")

			// queued -> failed
			err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusFailed, "first failure")
			require.NoError(t, err)

			// failed -> queued
			err = testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusQueued, "")
			require.NoError(t, err)

			doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
			require.NoError(t, err)
			require.Equal(t, model.ProcessingStatusQueued, doc.ProcessingStatus)
		})

		t.Run("recovery_failed_to_reprocessing_to_crawling", func(t *testing.T) {
			org := seedOrg(t, ctx, "recover-r-"+uuid.NewString()[:8])
			t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

			docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "recover2.md", "queued")

			// queued -> failed
			err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusFailed, "first failure")
			require.NoError(t, err)

			// failed -> reprocessing
			err = testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusReprocessing, "")
			require.NoError(t, err)

			// reprocessing -> crawling
			err = testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusCrawling, "")
			require.NoError(t, err)

			doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
			require.NoError(t, err)
			require.Equal(t, model.ProcessingStatusCrawling, doc.ProcessingStatus)
		})

		t.Run("reprocessing_from_ready", func(t *testing.T) {
			org := seedOrg(t, ctx, "reproc-"+uuid.NewString()[:8])
			t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

			docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "reproc.md", "queued")

			// Walk to ready
			for _, s := range []model.ProcessingStatus{
				model.ProcessingStatusCrawling,
				model.ProcessingStatusParsing,
				model.ProcessingStatusChunking,
				model.ProcessingStatusEmbedding,
				model.ProcessingStatusReady,
			} {
				require.NoError(t, testDocSvc.UpdateStatus(ctx, org.OrgID, docID, s, ""))
			}

			// ready -> reprocessing -> crawling -> parsing -> ...
			require.NoError(t, testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusReprocessing, ""))
			require.NoError(t, testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusCrawling, ""))
			require.NoError(t, testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusParsing, ""))

			doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
			require.NoError(t, err)
			require.Equal(t, model.ProcessingStatusParsing, doc.ProcessingStatus)
		})

		t.Run("invalid_transitions", func(t *testing.T) {
			org := seedOrg(t, ctx, "invalid-"+uuid.NewString()[:8])
			t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

			cases := []struct {
				name string
				to   model.ProcessingStatus
			}{
				{"queued_to_ready", model.ProcessingStatusReady},
				{"queued_to_parsing", model.ProcessingStatusParsing},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, tc.name+".md", "queued")
					err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, tc.to, "")
					require.Error(t, err, "transition from queued to %s should be rejected", tc.to)
				})
			}

			// ready -> queued is also invalid
			t.Run("ready_to_queued", func(t *testing.T) {
				docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "ready2queued.md", "queued")
				for _, s := range []model.ProcessingStatus{
					model.ProcessingStatusCrawling,
					model.ProcessingStatusParsing,
					model.ProcessingStatusChunking,
					model.ProcessingStatusEmbedding,
					model.ProcessingStatusReady,
				} {
					require.NoError(t, testDocSvc.UpdateStatus(ctx, org.OrgID, docID, s, ""))
				}
				err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusQueued, "")
				require.Error(t, err, "transition from ready to queued should be rejected")
			})
		})
	})

	t.Run("chunk_storage_correctness", func(t *testing.T) {
		org := seedOrg(t, ctx, "chunks-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "chunked.md", "ready")

		type chunkData struct {
			heading    string
			content    string
			tokenCount int
		}
		expected := []chunkData{
			{"Introduction", "This is the introduction section with overview content.", 120},
			{"Background", "Background information and context for the document.", 95},
			{"Methods", "Methodology section describing the approach taken.", 150},
			{"Results", "Results and findings from the analysis performed.", 200},
			{"Discussion", "Discussion of the results and their implications.", 180},
			{"Limitations", "Known limitations and constraints of the work.", 85},
			{"Future Work", "Potential future directions and improvements.", 110},
			{"Conclusion", "Summary and concluding remarks for the document.", 60},
		}

		for i, c := range expected {
			insertChunk(t, ctx, org.OrgID, org.KBID, docID, i, c.content, c.heading, c.tokenCount)
		}

		// Query chunks by document_id
		type chunkRow struct {
			heading    string
			index      int
			tokenCount int
			content    string
		}
		var rows []chunkRow

		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			r, err := tx.Query(ctx, `
				SELECT heading, chunk_index, token_count, content
				FROM chunks
				WHERE org_id = $1 AND document_id = $2
				ORDER BY chunk_index`, org.OrgID, docID)
			if err != nil {
				return err
			}
			defer r.Close()

			for r.Next() {
				var cr chunkRow
				if err := r.Scan(&cr.heading, &cr.index, &cr.tokenCount, &cr.content); err != nil {
					return err
				}
				rows = append(rows, cr)
			}
			return r.Err()
		})
		require.NoError(t, err)
		require.Len(t, rows, 8, "expected exactly 8 chunks")

		for i, row := range rows {
			require.Equal(t, expected[i].heading, row.heading, "chunk %d heading mismatch", i)
			require.Equal(t, i, row.index, "chunk %d index mismatch", i)
			require.Equal(t, expected[i].tokenCount, row.tokenCount, "chunk %d token_count mismatch", i)
			require.NotEmpty(t, row.content, "chunk %d content should not be empty", i)
		}
	})

	t.Run("embedding_storage", func(t *testing.T) {
		org := seedOrg(t, ctx, "embeds-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "embedded.md", "ready")

		chunkIDs := make([]string, 8)
		for i := 0; i < 8; i++ {
			chunkIDs[i] = insertChunk(t, ctx, org.OrgID, org.KBID, docID, i,
				fmt.Sprintf("Content for chunk %d", i), fmt.Sprintf("Heading %d", i), 100)
		}

		for i, cid := range chunkIDs {
			insertEmbedding(t, ctx, org.OrgID, cid, generateEmbedding(i))
		}

		// Query embeddings joined to chunks, assert 1:1 mapping
		var embCount int
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM embeddings e
				JOIN chunks c ON e.chunk_id = c.id
				WHERE c.org_id = $1 AND c.document_id = $2`, org.OrgID, docID).Scan(&embCount)
		})
		require.NoError(t, err)
		require.Equal(t, 8, embCount, "each chunk should have exactly one embedding")
	})

	t.Run("source_creation_all_types", func(t *testing.T) {
		org := seedOrg(t, ctx, "sources-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		sourceTypes := []struct {
			stype model.SourceType
			url   string
		}{
			// DB enum: url, sitemap, rss_feed (web_page/web_site not in enum yet)
			{model.SourceTypeSitemap, "https://example.com/sitemap.xml"},
			{model.SourceTypeRSSFeed, "https://example.com/feed.xml"},
		}

		for _, tc := range sourceTypes {
			t.Run(string(tc.stype), func(t *testing.T) {
				src, err := testSourceSvc.Create(ctx, org.OrgID, org.KBID, model.CreateSourceRequest{
					SourceType: tc.stype,
					URL:        tc.url,
				}, org.UserID)
				require.NoError(t, err)
				require.NotEmpty(t, src.ID)
				require.Equal(t, tc.stype, src.SourceType)
				require.Equal(t, tc.url, src.URL)
				require.Equal(t, org.OrgID, src.OrgID)
				require.Equal(t, org.KBID, src.KnowledgeBaseID)
			})
		}

		// Invalid source type should return error
		t.Run("invalid_type", func(t *testing.T) {
			_, err := testSourceSvc.Create(ctx, org.OrgID, org.KBID, model.CreateSourceRequest{
				SourceType: model.SourceType("invalid_type"),
				URL:        "https://example.com",
			}, org.UserID)
			require.Error(t, err, "invalid source type should be rejected")
		})
	})

	t.Run("duplicate_document_detection", func(t *testing.T) {
		org := seedOrg(t, ctx, "dedup-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		knownHash := "sha256:" + uuid.NewString()

		// Insert first document with a known hash
		doc1ID := uuid.NewString()
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO documents (id, org_id, knowledge_base_id, file_name, file_type, file_size_bytes, file_hash, storage_path, processing_status, uploaded_by)
				VALUES ($1, $2, $3, 'dup1.md', 'text/markdown', 2048, $4, '/test/dup1', 'queued'::processing_status, $5::uuid)`,
				doc1ID, org.OrgID, org.KBID, knownHash, org.UserID)
			return err
		})
		require.NoError(t, err)

		// Use FindByHash to detect the duplicate
		var found *model.Document
		err = db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			var err error
			found, err = testDocRepo.FindByHash(ctx, tx, org.OrgID, org.KBID, knownHash)
			return err
		})
		require.NoError(t, err)
		require.NotNil(t, found, "existing document with same hash should be found")
		require.Equal(t, doc1ID, found.ID)

		// Lookup with a different hash should return nil
		var notFound *model.Document
		err = db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			var err error
			notFound, err = testDocRepo.FindByHash(ctx, tx, org.OrgID, org.KBID, "sha256:nonexistent")
			return err
		})
		require.NoError(t, err)
		require.Nil(t, notFound, "no document should match a different hash")
	})

	t.Run("failure_path_with_error_message", func(t *testing.T) {
		org := seedOrg(t, ctx, "failmsg-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "failmsg.md", "queued")

		errMsg := "simulated parsing failure: out of memory"
		err := testDocSvc.UpdateStatus(ctx, org.OrgID, docID, model.ProcessingStatusFailed, errMsg)
		require.NoError(t, err)

		doc, err := testDocSvc.GetByID(ctx, org.OrgID, docID)
		require.NoError(t, err)
		require.Equal(t, model.ProcessingStatusFailed, doc.ProcessingStatus)
		require.Equal(t, errMsg, doc.ProcessingError)
	})

	t.Run("token_count_accuracy", func(t *testing.T) {
		org := seedOrg(t, ctx, "tokens-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "tokens.md", "ready")

		var expectedTotal int64
		for i := 0; i < 20; i++ {
			tc := (i + 1) * 50 // 50, 100, 150, ..., 1000
			insertChunk(t, ctx, org.OrgID, org.KBID, docID, i,
				fmt.Sprintf("Token count test chunk %d", i),
				fmt.Sprintf("Section %d", i), tc)
			expectedTotal += int64(tc)
		}

		var actualTotal int64
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT COALESCE(SUM(token_count), 0)
				FROM chunks
				WHERE org_id = $1 AND document_id = $2`, org.OrgID, docID).Scan(&actualTotal)
		})
		require.NoError(t, err)
		require.Equal(t, expectedTotal, actualTotal, "SUM(token_count) must match expected total of %d", expectedTotal)
	})

	t.Run("concurrent_ingestion", func(t *testing.T) {
		org := seedOrg(t, ctx, "concurrent-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		const numDocs = 2
		const chunksPerDoc = 10

		docIDs := make([]string, numDocs)
		for i := 0; i < numDocs; i++ {
			docIDs[i] = insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID,
				fmt.Sprintf("concurrent-%d.md", i), "ready")
		}

		g, _ := errgroup.WithContext(ctx)

		for docIdx := 0; docIdx < numDocs; docIdx++ {
			docIdx := docIdx
			g.Go(func() error {
				for j := 0; j < chunksPerDoc; j++ {
					chunkID := uuid.NewString()
					if err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
						_, err := tx.Exec(ctx, `
							INSERT INTO chunks (id, org_id, knowledge_base_id, document_id, content, chunk_index, token_count, page_number, heading, chunk_type)
							VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, 'text')`,
							chunkID, org.OrgID, org.KBID, docIDs[docIdx],
							fmt.Sprintf("Doc %d chunk %d content", docIdx, j),
							j, 100, fmt.Sprintf("Heading %d-%d", docIdx, j))
						return err
					}); err != nil {
						return fmt.Errorf("insert chunk doc=%d chunk=%d: %w", docIdx, j, err)
					}
					if err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
						_, err := tx.Exec(ctx, `
							INSERT INTO embeddings (id, org_id, chunk_id, embedding, model_name, dimensions)
							VALUES ($1, $2, $3, $4::vector, 'text-embedding-3-small', 1536)`,
							uuid.NewString(), org.OrgID, chunkID, vectorToString(generateEmbedding(docIdx*100+j)))
						return err
					}); err != nil {
						return fmt.Errorf("insert embedding doc=%d chunk=%d: %w", docIdx, j, err)
					}
				}
				return nil
			})
		}

		err := g.Wait()
		require.NoError(t, err, "concurrent ingestion should complete without errors")

		// Verify both documents have the correct number of chunks
		for i, docID := range docIDs {
			var count int
			err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
				return tx.QueryRow(ctx, `
					SELECT COUNT(*) FROM chunks
					WHERE org_id = $1 AND document_id = $2`, org.OrgID, docID).Scan(&count)
			})
			require.NoError(t, err)
			require.Equal(t, chunksPerDoc, count, "document %d should have %d chunks", i, chunksPerDoc)
		}
	})

	t.Run("large_document_500_chunks", func(t *testing.T) {
		org := seedOrg(t, ctx, "large-"+uuid.NewString()[:8])
		t.Cleanup(func() { cleanupOrg(t, ctx, org.OrgID) })

		docID := insertDocument(t, ctx, org.OrgID, org.KBID, org.UserID, "large.md", "ready")

		const numChunks = 500
		for i := 0; i < numChunks; i++ {
			insertChunk(t, ctx, org.OrgID, org.KBID, docID, i,
				fmt.Sprintf("Large document chunk %d with sufficient content to be realistic.", i),
				fmt.Sprintf("Section %d", i), 100+i)
		}

		var count int
		err := db.WithOrgID(ctx, testPool, org.OrgID, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `
				SELECT COUNT(*) FROM chunks
				WHERE org_id = $1 AND document_id = $2`, org.OrgID, docID).Scan(&count)
		})
		require.NoError(t, err)
		require.Equal(t, numChunks, count, "all 500 chunks should be queryable")
	})
}
