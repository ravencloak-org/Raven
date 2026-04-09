# Go Backend Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write missing Go unit and integration tests for all backend packages to reach 80% line coverage, covering handlers, services, repositories (with RLS), middleware, gRPC client, queue/jobs, crypto, cache, storage, and all EE features.

**Architecture:** Manual mock structs with function fields (existing pattern), testify for assertions, testcontainers-go for real PostgreSQL integration tests, miniredis for Valkey, httptest for HTTP. Tests co-located with source as `*_test.go` files.

**Tech Stack:** Go 1.25, `github.com/stretchr/testify v1.11.1`, `github.com/testcontainers/testcontainers-go v0.41.0`, `github.com/alicebob/miniredis/v2 v2.37.0`, `github.com/gin-gonic/gin v1.12.0`, `github.com/jackc/pgx/v5`

---

## Pre-flight: Audit Existing Tests

- [ ] **Step 1: List existing test files to find gaps**

```bash
find /Users/jobinlawrance/Project/raven/internal -name '*_test.go' | sort
```

Note which packages have zero or minimal test files. Focus new work on those. Do not rewrite passing tests.

---

## Task 1: Test Helpers (`internal/testutil/`)

**Files:**
- Create: `internal/testutil/db.go`
- Create: `internal/testutil/fixtures.go`
- Create: `internal/testutil/grpc.go`

- [ ] **Step 1: Create `internal/testutil/db.go`**

```go
package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestDB spins up a real PostgreSQL container, runs all migrations,
// and returns a pool. Container is terminated when t ends.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("raven_test"),
		postgres.WithUsername("raven"),
		postgres.WithPassword("raven"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	RunMigrations(t, connStr)
	return pool
}

// RunMigrations applies all goose migrations from the repo migrations/ dir.
func RunMigrations(t *testing.T, connStr string) {
	t.Helper()
	// Use goose programmatic API
	// import "github.com/pressly/goose/v3"
	// goose.SetBaseFS(os.DirFS("../../migrations"))
	// db, _ := sql.Open("pgx", connStr)
	// require.NoError(t, goose.Up(db, "."))
	// Adjust relative path from internal/testutil/ to migrations/
}
```

- [ ] **Step 2: Create `internal/testutil/fixtures.go`**

```go
package testutil

import (
	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/internal/model"
)

func NewOrg(overrides ...func(*model.Organization)) *model.Organization {
	org := &model.Organization{
		ID:   uuid.NewString(),
		Name: "Test Org",
		Slug: "test-org",
	}
	for _, o := range overrides { o(org) }
	return org
}

func NewWorkspace(orgID string, overrides ...func(*model.Workspace)) *model.Workspace {
	ws := &model.Workspace{
		ID:    uuid.NewString(),
		OrgID: orgID,
		Name:  "Test Workspace",
	}
	for _, o := range overrides { o(ws) }
	return ws
}

func NewKnowledgeBase(workspaceID string) *model.KnowledgeBase {
	return &model.KnowledgeBase{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        "Test KB",
	}
}

func NewAPIKey(workspaceID, kbID string) *model.APIKey {
	return &model.APIKey{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		KBID:        kbID,
		KeyHash:     "testhash",
	}
}
```

- [ ] **Step 3: Create `internal/testutil/grpc.go` — stub gRPC AI Worker**

```go
package testutil

import (
	"context"
	"io"

	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"google.golang.org/grpc"
)

// StubAIWorker is a deterministic gRPC AI worker stub for tests.
type StubAIWorker struct {
	ParseAndEmbedFn func(ctx context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error)
	QueryRAGFn      func(req *pb.RAGRequest, stream pb.AIWorker_QueryRAGClient) error
	GetEmbeddingFn  func(ctx context.Context, req *pb.EmbeddingRequest) (*pb.EmbeddingResponse, error)
}

func (s *StubAIWorker) ParseAndEmbed(ctx context.Context, req *pb.ParseRequest, opts ...grpc.CallOption) (*pb.ParseResponse, error) {
	if s.ParseAndEmbedFn != nil { return s.ParseAndEmbedFn(ctx, req) }
	return &pb.ParseResponse{ChunkCount: 3}, nil
}

func (s *StubAIWorker) GetEmbedding(ctx context.Context, req *pb.EmbeddingRequest, opts ...grpc.CallOption) (*pb.EmbeddingResponse, error) {
	if s.GetEmbeddingFn != nil { return s.GetEmbeddingFn(ctx, req) }
	vec := make([]float32, 1536)
	return &pb.EmbeddingResponse{Embedding: vec, Dimensions: 1536}, nil
}
```

- [ ] **Step 4: Commit helpers**

```bash
git add internal/testutil/
git commit -m "test: add shared test helpers (DB, fixtures, gRPC stub)"
```

---

## Task 2: Handler Unit Tests

**Files:**
- Modify/Create: `internal/handler/chat_test.go`
- Modify/Create: `internal/handler/kb_test.go`
- Modify/Create: `internal/handler/document_test.go`
- Modify/Create: `internal/handler/apikey_test.go`
- Modify/Create: `internal/handler/meta_webhook_test.go`

- [ ] **Step 1: Write failing handler test for chat endpoint**

Check if `internal/handler/chat_test.go` exists. If not, create it. Follow the pattern in `internal/handler/user_test.go` exactly: manual mock struct with function fields, `httptest.NewRecorder`, inject context values via middleware.

```go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/model"
)

type mockChatService struct {
	SendMessageFn func(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error)
}

func (m *mockChatService) SendMessage(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
	return m.SendMessageFn(ctx, req)
}

func newChatTestRouter(svc handler.ChatServiceIface) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("org_id", "test-org-id")
		c.Set("workspace_id", "test-ws-id")
		c.Next()
	})
	// Register routes — adjust to actual registration function name
	h := handler.NewChatHandler(svc)
	r.POST("/api/v1/chat", h.SendMessage)
	return r
}

func TestChatHandler_SendMessage_Success(t *testing.T) {
	svc := &mockChatService{
		SendMessageFn: func(ctx context.Context, req *model.ChatRequest) (*model.ChatResponse, error) {
			return &model.ChatResponse{Message: "Hello"}, nil
		},
	}
	router := newChatTestRouter(svc)

	body, _ := json.Marshal(map[string]string{"message": "hi", "kb_id": "kb-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Hello", resp["message"])
}

func TestChatHandler_SendMessage_MissingBody(t *testing.T) {
	svc := &mockChatService{}
	router := newChatTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run to verify it compiles and fails correctly**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/handler/... -run TestChatHandler -v 2>&1 | head -30
```

Adapt mock interface names to match actual interface definitions in `internal/handler/`. Run `grep -r "interface" internal/handler/*.go` to find actual interface names.

- [ ] **Step 3: Add KB handler tests** (`internal/handler/kb_test.go`)

Test cases:
- `TestKBHandler_Create_Success` — valid body → 201
- `TestKBHandler_Create_MissingName` — empty name → 400
- `TestKBHandler_List_Success` — returns list → 200
- `TestKBHandler_Delete_NotFound` — service returns not found → 404

Follow same mock + router pattern.

- [ ] **Step 4: Add document handler tests** (`internal/handler/document_test.go`)

Test cases:
- `TestDocumentHandler_Upload_Success` — multipart file → 202 (processing started)
- `TestDocumentHandler_Upload_UnsupportedType` — .exe file → 415
- `TestDocumentHandler_AddURL_Success` — valid URL → 202
- `TestDocumentHandler_List_Success` — returns paginated list → 200

- [ ] **Step 5: Add API key handler tests** (`internal/handler/apikey_test.go`)

Test cases:
- `TestAPIKeyHandler_Create_WorkspaceScoped` → 201 with key value in response
- `TestAPIKeyHandler_Create_KBScoped` → 201
- `TestAPIKeyHandler_Revoke_Success` → 204
- `TestAPIKeyHandler_List_Success` → 200 with array

- [ ] **Step 6: Add Meta webhook handler test** (`internal/handler/meta_webhook_test.go`)

Test cases:
- `TestMetaWebhookHandler_ValidHMAC` — correct `X-Hub-Signature-256` → 200
- `TestMetaWebhookHandler_InvalidHMAC` — wrong signature → 403
- `TestMetaWebhookHandler_VerificationChallenge` — GET with hub.challenge → echo challenge

- [ ] **Step 7: Run all handler tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/handler/... -v -count=1 2>&1 | tail -20
```

Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add internal/handler/*_test.go
git commit -m "test: handler unit tests (chat, KB, document, apikey, webhook)"
```

---

## Task 3: Service Unit Tests

**Files:**
- Modify/Create: `internal/service/chat_test.go`
- Modify/Create: `internal/service/search_test.go`
- Modify/Create: `internal/service/document_test.go`
- Modify/Create: `internal/service/security_test.go`
- Modify/Create: `internal/service/voice_test.go`
- Modify/Create: `internal/service/whatsapp_test.go`

- [ ] **Step 1: Audit which service test files exist**

```bash
ls internal/service/*_test.go 2>/dev/null || echo "none"
```

- [ ] **Step 2: Write failing ChatService tests**

```go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

type mockChatRepo struct {
	SaveMessageFn func(ctx context.Context, msg *model.Message) error
	GetHistoryFn  func(ctx context.Context, sessionID string) ([]*model.Message, error)
}
func (m *mockChatRepo) SaveMessage(ctx context.Context, msg *model.Message) error {
	return m.SaveMessageFn(ctx, msg)
}
func (m *mockChatRepo) GetHistory(ctx context.Context, sessionID string) ([]*model.Message, error) {
	return m.GetHistoryFn(ctx, sessionID)
}

func TestChatService_SendMessage_SavesMessage(t *testing.T) {
	saved := false
	repo := &mockChatRepo{
		SaveMessageFn: func(ctx context.Context, msg *model.Message) error {
			saved = true
			assert.Equal(t, "user", msg.Role)
			return nil
		},
		GetHistoryFn: func(ctx context.Context, sessionID string) ([]*model.Message, error) {
			return nil, nil
		},
	}
	stub := testutil.StubAIWorker{}
	svc := service.NewChatService(repo, &stub)

	_, err := svc.SendMessage(context.Background(), &model.ChatRequest{
		Message:   "hello",
		SessionID: "session-1",
		KBID:      "kb-1",
		OrgID:     "org-1",
	})
	require.NoError(t, err)
	assert.True(t, saved)
}
```

Adapt field names and constructors to match actual `service.NewChatService` signature. Run `grep -n "func NewChatService" internal/service/chat.go` to confirm.

- [ ] **Step 3: Write SearchService hybrid retrieval tests**

```go
func TestSearchService_HybridSearch_ReturnsMergedResults(t *testing.T) {
	// Mock vector results and BM25 results
	// Verify RRF fusion returns ranked slice
	// Assert len(results) > 0 and first result has highest score
}

func TestSearchService_HybridSearch_EmptyKB_ReturnsEmpty(t *testing.T) {
	// Both vector and BM25 return empty
	// Assert results == []
}
```

- [ ] **Step 4: Write SecurityService WAF tests**

```go
func TestSecurityService_EvalRule_BlockPattern_Returns403(t *testing.T) {
	// Create block rule for pattern "DROP TABLE"
	// Call EvalRules with matching request
	// Assert action == "block", status == 403
}

func TestSecurityService_EvalRule_AllowOverridesBlock(t *testing.T) {
	// Block rule + allow rule for same pattern, allow has higher priority
	// Assert action == "allow"
}

func TestSecurityService_EvalRule_LogRule_PassesThrough(t *testing.T) {
	// Log-only rule
	// Assert action == "log" and request not blocked
}
```

- [ ] **Step 5: Write VoiceService session lifecycle tests**

```go
func TestVoiceService_CreateSession_ReturnsToken(t *testing.T) {}
func TestVoiceService_EndSession_MarksComplete(t *testing.T) {}
func TestVoiceService_GetActiveSessions_ReturnsOnlyActive(t *testing.T) {}
```

- [ ] **Step 6: Write WhatsAppService webhook dispatch tests**

```go
func TestWhatsAppService_HandleEvent_TextMessage_EnqueuesJob(t *testing.T) {}
func TestWhatsAppService_HandleEvent_UnknownType_Ignored(t *testing.T) {}
```

- [ ] **Step 7: Run service tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/service/... -v -count=1 2>&1 | tail -30
```

- [ ] **Step 8: Commit**

```bash
git add internal/service/*_test.go
git commit -m "test: service unit tests (chat, search, security, voice, whatsapp)"
```

---

## Task 4: Repository Integration Tests (testcontainers)

**Files:**
- Modify/Create: `internal/repository/kb_test.go`
- Modify/Create: `internal/repository/document_test.go`
- Modify/Create: `internal/repository/chat_test.go`
- Modify/Create: `internal/repository/apikey_test.go`
- Create: `internal/repository/rls_test.go`

- [ ] **Step 1: Write a failing KB repository test using testcontainers**

```go
package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/testutil"
)

func TestKBRepository_Create_And_Get(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := repository.NewKBRepository(pool)
	ctx := context.Background()

	kb := testutil.NewKnowledgeBase("ws-test-1")
	err := repo.Create(ctx, kb)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, kb.ID)
	require.NoError(t, err)
	assert.Equal(t, kb.Name, got.Name)
}

func TestKBRepository_List_PaginationWorks(t *testing.T) {
	pool := testutil.NewTestDB(t)
	repo := repository.NewKBRepository(pool)
	ctx := context.Background()

	// Insert 5 KBs
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Create(ctx, testutil.NewKnowledgeBase("ws-page-test")))
	}

	results, err := repo.List(ctx, "ws-page-test", 3, 0)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}
```

- [ ] **Step 2: Run to verify fails (testcontainers starts PG)**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/repository/... -run TestKBRepository -v -timeout 120s 2>&1 | tail -20
```

- [ ] **Step 3: Write RLS cross-tenant isolation tests** (`internal/repository/rls_test.go`)

This is the most critical integration test. Two orgs must never see each other's data.

```go
func TestRLS_CrossOrgRead_ReturnsZeroRows(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	// Insert KB for org-A
	kbRepo := repository.NewKBRepository(pool)
	orgAKB := testutil.NewKnowledgeBase("ws-org-a")
	orgAKB.OrgID = "org-a"
	require.NoError(t, kbRepo.Create(ctx, orgAKB))

	// Set RLS context to org-B
	ctxOrgB := context.WithValue(ctx, "org_id", "org-b")

	// Query as org-B — must return empty
	results, err := kbRepo.List(ctxOrgB, "ws-org-a", 100, 0)
	require.NoError(t, err)
	assert.Empty(t, results, "org-B must not see org-A's knowledge bases")
}

func TestRLS_CrossOrgChat_ReturnsZeroRows(t *testing.T) {
	pool := testutil.NewTestDB(t)
	// Same pattern for chat sessions
}

func TestRLS_CrossOrgDocument_ReturnsZeroRows(t *testing.T) {
	pool := testutil.NewTestDB(t)
	// Same pattern for documents
}
```

Adjust how org context is set in the repository queries — look at `internal/repository/*.go` to see how the current RLS context is passed (likely via `pgx` connection config or a middleware).

- [ ] **Step 4: Write API key repository tests**

```go
func TestAPIKeyRepository_Create_And_ValidateByHash(t *testing.T) {}
func TestAPIKeyRepository_Revoke_MarksInactive(t *testing.T) {}
func TestAPIKeyRepository_ValidateScope_KBScoped_BlocksOtherKB(t *testing.T) {}
```

- [ ] **Step 5: Run all repository tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/repository/... -v -timeout 180s -count=1 2>&1 | tail -30
```

- [ ] **Step 6: Commit**

```bash
git add internal/repository/*_test.go
git commit -m "test: repository integration tests with RLS cross-tenant isolation"
```

---

## Task 5: Middleware Unit Tests

**Files:**
- Modify/Create: `internal/middleware/auth_test.go`
- Modify/Create: `internal/middleware/ratelimit_test.go`
- Modify/Create: `internal/middleware/security_test.go`

- [ ] **Step 1: Write JWT auth middleware tests**

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

const testJWTSecret = "test-secret-key"

func makeJWT(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil { t.Fatal(err) }
	return signed
}

func TestAuthMiddleware_ValidJWT_SetsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.JWTAuth(testJWTSecret))
	r.GET("/test", func(c *gin.Context) {
		orgID, _ := c.Get("org_id")
		c.JSON(200, gin.H{"org_id": orgID})
	})

	token := makeJWT(t, jwt.MapClaims{
		"org_id": "org-123",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func TestAuthMiddleware_ExpiredJWT_Returns401(t *testing.T) {
	// Same setup, token with exp in the past
	// Assert 401
}

func TestAuthMiddleware_NoHeader_Returns401(t *testing.T) {
	// No Authorization header
	// Assert 401
}

func TestAuthMiddleware_WrongAudience_Returns401(t *testing.T) {
	// JWT with wrong aud claim
	// Assert 401
}
```

- [ ] **Step 2: Write API key auth middleware tests**

```go
func TestAPIKeyMiddleware_ValidKey_SetsContext(t *testing.T) {}
func TestAPIKeyMiddleware_RevokedKey_Returns401(t *testing.T) {}
func TestAPIKeyMiddleware_WrongScope_Returns403(t *testing.T) {}
```

- [ ] **Step 3: Write rate limiter middleware tests**

```go
func TestRateLimiter_UnderThreshold_Passes(t *testing.T) {
	// Send N-1 requests, all should be 200
}
func TestRateLimiter_AtThreshold_Returns429(t *testing.T) {
	// Send N+1 requests, last should be 429 with Retry-After header
	// Assert w.Header().Get("Retry-After") != ""
}
```

- [ ] **Step 4: Run middleware tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/middleware/... -v -count=1 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add internal/middleware/*_test.go
git commit -m "test: middleware unit tests (JWT, API key, rate limiter)"
```

---

## Task 6: gRPC Client Tests with Fault Injection

**Files:**
- Modify/Create: `internal/grpc/client_test.go`

- [ ] **Step 1: Write gRPC client tests covering happy path and faults**

```go
package grpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// startFakeServer starts an in-process gRPC server for testing.
func startFakeServer(t *testing.T, impl pb.AIWorkerServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := grpc.NewServer()
	pb.RegisterAIWorkerServer(s, impl)
	go s.Serve(lis)
	t.Cleanup(s.Stop)
	return lis.Addr().String()
}

func TestGRPCClient_ParseAndEmbed_Success(t *testing.T) {
	addr := startFakeServer(t, &fakeAIWorker{})
	client := grpc.NewRavenGRPCClient(addr) // adjust to actual constructor
	resp, err := client.ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: "hello"})
	require.NoError(t, err)
	assert.Greater(t, resp.ChunkCount, int32(0))
}

func TestGRPCClient_ConnectionRefused_ReturnsWrappedError(t *testing.T) {
	client := grpc.NewRavenGRPCClient("127.0.0.1:1") // nothing listening
	_, err := client.ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: "test"})
	assert.Error(t, err)
	// Should be wrapped, not raw gRPC error
	assert.NotContains(t, err.Error(), "rpc error")
}

func TestGRPCClient_Unavailable_PropagatesAs503(t *testing.T) {
	addr := startFakeServer(t, &fakeAIWorker{
		parseErr: status.Error(codes.Unavailable, "worker down"),
	})
	client := grpc.NewRavenGRPCClient(addr)
	_, err := client.ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503") // verify HTTP-layer mapping
}

func TestGRPCClient_ResourceExhausted_PropagatesAs429(t *testing.T) {
	addr := startFakeServer(t, &fakeAIWorker{
		parseErr: status.Error(codes.ResourceExhausted, "rate limit"),
	})
	client := grpc.NewRavenGRPCClient(addr)
	_, err := client.ParseAndEmbed(context.Background(), &pb.ParseRequest{Content: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestGRPCClient_QueryRAG_StreamAssembly(t *testing.T) {
	// Fake server yields 3 chunks
	// Assert assembled response has all chunks concatenated
}

func TestGRPCClient_Timeout_Cancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	// Fake server that sleeps before responding
	// Assert context.DeadlineExceeded
}
```

- [ ] **Step 2: Run**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/grpc/... -v -count=1 2>&1 | tail -20
```

- [ ] **Step 3: Commit**

```bash
git add internal/grpc/*_test.go
git commit -m "test: gRPC client tests including fault injection (connection refused, UNAVAILABLE, RESOURCE_EXHAUSTED)"
```

---

## Task 7: Queue & Jobs Tests

**Files:**
- Modify/Create: `internal/queue/queue_test.go`
- Modify/Create: `internal/jobs/send_email_test.go`
- Modify/Create: `internal/jobs/webhook_delivery_test.go`

- [ ] **Step 1: Write queue enqueue tests using miniredis**

```go
package queue_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ravencloak-org/Raven/internal/queue"
)

func TestEnqueueDocumentProcessing_SerializesPayload(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	q := queue.New(client)

	err := q.EnqueueDocumentProcessing(context.Background(), "doc-123", "org-abc")
	require.NoError(t, err)

	// Asynq stores tasks as Redis hashes — inspect that task was enqueued
	keys := mr.Keys()
	assert.NotEmpty(t, keys)
}
```

- [ ] **Step 2: Write webhook delivery job handler test (verifies retry intervals)**

```go
func TestWebhookDeliveryHandler_Retry_BackoffIntervals(t *testing.T) {
	attempts := []time.Duration{}
	// Mock HTTP client that always returns 500
	// Run handler 4 times, record delay between calls
	// Assert delays approximate [1s, 5s, 30s]
}
```

- [ ] **Step 3: Run**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/queue/... ./internal/jobs/... -v -count=1 2>&1 | tail -20
```

- [ ] **Step 4: Commit**

```bash
git add internal/queue/*_test.go internal/jobs/*_test.go
git commit -m "test: queue enqueue and job handler tests with retry backoff verification"
```

---

## Task 8: Crypto, Cache & Storage Tests

**Files:**
- Modify/Create: `internal/crypto/crypto_test.go`
- Check existing: `internal/cache/cache_test.go` (already partially written — audit gaps only)
- Modify/Create: `internal/storage/storage_test.go`

- [ ] **Step 1: Crypto tests**

```go
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	plaintext := "sensitive-api-key-value"
	encrypted, err := crypto.Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := crypto.Decrypt(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestHashAPIKey_DeterministicAndNonReversible(t *testing.T) {
	hash1 := crypto.HashAPIKey("my-key")
	hash2 := crypto.HashAPIKey("my-key")
	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, "my-key", hash1)
}
```

- [ ] **Step 2: Storage tests with cross-tenant isolation**

```go
func TestStorageClient_CrossTenantAccess_ReturnsForbidden(t *testing.T) {
	// Mock HTTP server that returns 403 for cross-tenant path pattern
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "org-b") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := storage.NewSeaweedFSClient(server.URL)
	err := client.Download(context.Background(), "org-b", "file-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}
```

- [ ] **Step 3: Run**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/crypto/... ./internal/cache/... ./internal/storage/... -v -count=1 2>&1 | tail -20
```

- [ ] **Step 4: Commit**

```bash
git add internal/crypto/*_test.go internal/storage/*_test.go
git commit -m "test: crypto round-trip, storage cross-tenant isolation"
```

---

## Task 9: Integration Tests — Document Pipeline, Hybrid Search, SSE

**Files:**
- Create: `internal/integration/document_pipeline_test.go`
- Create: `internal/integration/hybrid_search_test.go`
- Create: `internal/integration/sse_chat_test.go`
- Create: `internal/integration/webhook_delivery_test.go`

- [ ] **Step 1: Write document processing pipeline integration test**

```go
package integration_test

func TestDocumentPipeline_UploadToRetrievable(t *testing.T) {
	pool := testutil.NewTestDB(t)
	mr := miniredis.RunT(t)
	stub := &testutil.StubAIWorker{} // returns 3 chunks with fake embeddings
	
	// Wire up services
	docSvc := service.NewDocumentService(
		repository.NewDocumentRepository(pool),
		repository.NewChunkRepository(pool),
		stub,
		queue.New(redis.NewClient(&redis.Options{Addr: mr.Addr()})),
	)

	doc, err := docSvc.Upload(context.Background(), &model.UploadRequest{
		Content:     []byte("Test document content"),
		Filename:    "test.txt",
		ContentType: "text/plain",
		OrgID:       "org-int",
		KBID:        "kb-int",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, doc.ID)

	// Trigger processing synchronously in test
	err = docSvc.ProcessDocument(context.Background(), doc.ID)
	require.NoError(t, err)

	// Verify chunks stored
	chunks, err := repository.NewChunkRepository(pool).ListByDocument(context.Background(), doc.ID)
	require.NoError(t, err)
	assert.Len(t, chunks, 3)
}
```

- [ ] **Step 2: Write hybrid search integration test**

```go
func TestHybridSearch_ReturnsRankedResults(t *testing.T) {
	pool := testutil.NewTestDB(t)
	// Insert 5 chunks with known vectors
	// Insert matching BM25 index entries
	// Call SearchService.Search with a query
	// Assert top result matches expected chunk
}
```

- [ ] **Step 3: Write SSE streaming test**

```go
func TestSSEChat_StreamingResponseArrives(t *testing.T) {
	// Start test HTTP server with the chat route
	// Send request with Accept: text/event-stream
	// Read response body as event stream
	// Assert data: events arrive and assemble to non-empty text
}
```

- [ ] **Step 4: Run integration tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/integration/... -v -timeout 300s -count=1 2>&1 | tail -30
```

- [ ] **Step 5: Commit**

```bash
git add internal/integration/
git commit -m "test: integration tests for document pipeline, hybrid search, SSE streaming"
```

---

## Task 10: Enterprise Edition Tests

**Files:**
- Create: `internal/ee/security/*_test.go` — WAF rule engine
- Create: `internal/ee/webhooks/*_test.go` — webhook delivery
- Create: `internal/ee/licensing/*_test.go` — feature gates
- Create: `internal/ee/sso/*_test.go` — Keycloak token exchange
- Create: `internal/ee/connectors/*_test.go` — Airbyte sync
- Create: `internal/ee/analytics/*_test.go` — event writes
- Create: `internal/ee/audit/*_test.go` — audit log capture

First, audit the actual EE package structure:

```bash
find internal/ee -type f -name '*.go' | grep -v _test | sort
```

Then write tests following the same mock + testify pattern. Key cases per package:

**`internal/ee/licensing/`**
```go
func TestLicenseGuard_NoLicense_Returns402(t *testing.T) {}
func TestLicenseGuard_ValidLicense_Passes(t *testing.T) {}
func TestLicenseGuard_ExpiredLicense_Returns402(t *testing.T) {}
func TestLicenseGuard_FeatureADoesNotUnlockFeatureB(t *testing.T) {}
```

**`internal/ee/security/`**
```go
func TestWAFEngine_BlockRule_MatchingRequest_Blocked(t *testing.T) {}
func TestWAFEngine_AllowRule_OverridesBlock(t *testing.T) {}
func TestWAFEngine_LogRule_PassesThrough_CreatesAuditEntry(t *testing.T) {}
func TestWAFEngine_RulePriority_HigherWins(t *testing.T) {}
func TestWAFEngine_RegexRule_MatchingAndNonMatching(t *testing.T) {}
```

**`internal/ee/webhooks/`**
```go
func TestWebhookDelivery_HMACSignature_Correct(t *testing.T) {}
func TestWebhookDelivery_Retry_Intervals_1s_5s_30s(t *testing.T) {}
func TestWebhookDelivery_DeadLetter_AfterMaxRetries(t *testing.T) {}
func TestWebhookDelivery_Replay_DeadLettered(t *testing.T) {}
```

**`internal/ee/audit/`**
```go
func TestAuditLog_Create_CapturesCorrectFields(t *testing.T) {}
func TestAuditLog_Delete_EntryRetained(t *testing.T) {}
func TestAuditLog_Query_FilterByActor(t *testing.T) {}
```

- [ ] **Step 1: Run audit of EE package structure, write tests per package**

- [ ] **Step 2: Run EE tests**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./internal/ee/... -v -count=1 2>&1 | tail -40
```

- [ ] **Step 3: Commit**

```bash
git add internal/ee/
git commit -m "test: enterprise edition tests (WAF, licensing, webhooks, audit, SSO, analytics)"
```

---

## Task 11: Coverage Gate Verification

- [ ] **Step 1: Run full test suite with coverage**

```bash
cd /Users/jobinlawrance/Project/raven && go test ./... -coverprofile=coverage.out -timeout 300s -count=1 2>&1 | tail -10
```

- [ ] **Step 2: Check coverage**

```bash
go tool cover -func=coverage.out | grep total
```

Expected: `total: (statements) 80.0%` or higher

- [ ] **Step 3: If below 80%, identify gaps**

```bash
go tool cover -func=coverage.out | sort -k3 -n | head -20
```

Write additional tests for lowest-coverage packages.

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "test: achieve 80% Go coverage gate"
```
