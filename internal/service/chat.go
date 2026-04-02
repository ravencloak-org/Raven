package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	rpcClient "github.com/ravencloak-org/Raven/internal/grpc"
	pb "github.com/ravencloak-org/Raven/internal/grpc/pb"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SSEEventType describes the kind of SSE event being sent.
type SSEEventType string

const (
	// SSEEventToken is emitted for each streamed text token.
	SSEEventToken SSEEventType = "token"
	// SSEEventSources is emitted when the AI worker returns source chunks.
	SSEEventSources SSEEventType = "sources"
	// SSEEventDone signals that the stream is complete.
	SSEEventDone SSEEventType = "done"
	// SSEEventError signals an error during streaming.
	SSEEventError SSEEventType = "error"
)

// SSEEvent represents a single Server-Sent Event to be sent to the client.
type SSEEvent struct {
	Event SSEEventType `json:"event"`
	Data  any          `json:"data"`
}

// ChatRepository defines the interface the chat service requires for persistence.
type ChatRepository interface {
	CreateSession(ctx context.Context, tx pgx.Tx, session *model.ChatSession) (*model.ChatSession, error)
	GetSession(ctx context.Context, tx pgx.Tx, sessionID, orgID string) (*model.ChatSession, error)
	CreateMessage(ctx context.Context, tx pgx.Tx, msg *model.ChatMessage) (*model.ChatMessage, error)
	ListMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit int) ([]model.ChatMessage, error)
	GetOrCreateSession(ctx context.Context, tx pgx.Tx, orgID, kbID, sessionID string, userID *string) (*model.ChatSession, bool, error)
	ListRecentMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit int) ([]model.ChatMessage, error)
	CountSessionMessages(ctx context.Context, tx pgx.Tx, sessionID, orgID string) (int, error)
	DeleteExpiredSessions(ctx context.Context, tx pgx.Tx) (int64, error)
	UpdateSessionExpiry(ctx context.Context, tx pgx.Tx, sessionID, orgID string) error
	ListSessions(ctx context.Context, tx pgx.Tx, orgID, kbID string, limit, offset int) ([]model.ChatSession, error)
	DeleteSession(ctx context.Context, tx pgx.Tx, sessionID, orgID string) error
	ListMessagesPaged(ctx context.Context, tx pgx.Tx, sessionID, orgID string, limit, offset int) ([]model.ChatMessage, error)
}

// AIWorkerClient defines the gRPC interface needed by the chat service.
type AIWorkerClient interface {
	Worker() pb.AIWorkerClient
}

// ChatService contains business logic for chat completions with SSE streaming.
type ChatService struct {
	chatRepo   ChatRepository
	grpcClient AIWorkerClient
	pool       *pgxpool.Pool
}

// NewChatService creates a new ChatService.
func NewChatService(chatRepo *repository.ChatRepository, grpcClient *rpcClient.Client, pool *pgxpool.Pool) *ChatService {
	return &ChatService{chatRepo: chatRepo, grpcClient: grpcClient, pool: pool}
}

// NewChatServiceWithDeps creates a ChatService with explicit interface dependencies (for testing).
func NewChatServiceWithDeps(chatRepo ChatRepository, grpcClient AIWorkerClient, pool *pgxpool.Pool) *ChatService {
	return &ChatService{chatRepo: chatRepo, grpcClient: grpcClient, pool: pool}
}

// StreamCompletion calls QueryRAG and returns a channel of SSE events.
// It creates or looks up a session, builds conversation context, persists
// the user message, streams the gRPC response, and persists the assistant
// reply when complete.
func (s *ChatService) StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan SSEEvent, error) {
	var session *model.ChatSession
	var userMsg *model.ChatMessage
	var convCtx *model.ConversationContext

	// Determine session ID — use provided or generate a new one.
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Step 1: Get or create session, build conversation context, save user message.
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		var isCreated bool
		session, isCreated, err = s.chatRepo.GetOrCreateSession(ctx, tx, orgID, kbID, sessionID, nil)
		if err != nil {
			return fmt.Errorf("get or create session: %w", err)
		}

		// Build conversation context from existing messages (only if session already existed).
		if !isCreated {
			convCtx, err = s.buildConversationContextTx(ctx, tx, orgID, session.ID, model.DefaultMaxTurns, model.DefaultTokenBudget)
			if err != nil {
				return fmt.Errorf("build conversation context: %w", err)
			}
		} else {
			convCtx = &model.ConversationContext{
				SessionID: session.ID,
				Messages:  nil,
			}
		}

		// Save user message with estimated token count.
		userTokens := model.EstimateTokens(req.Query)
		userMsg, err = s.chatRepo.CreateMessage(ctx, tx, &model.ChatMessage{
			SessionID:  session.ID,
			OrgID:      orgID,
			Role:       "user",
			Content:    req.Query,
			TokenCount: &userTokens,
		})
		if err != nil {
			return fmt.Errorf("save user message: %w", err)
		}

		// Refresh TTL for anonymous sessions.
		if session.ExpiresAt != nil {
			if err := s.chatRepo.UpdateSessionExpiry(ctx, tx, session.ID, orgID); err != nil {
				return fmt.Errorf("update session expiry: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to initialise chat: " + err.Error())
	}

	// Step 2: Build filters with conversation context.
	filters := req.Filters
	if filters == nil {
		filters = map[string]string{}
	}
	if convCtx != nil && len(convCtx.Messages) > 0 {
		ctxJSON, err := json.Marshal(convCtx)
		if err == nil {
			filters["conversation_context"] = string(ctxJSON)
		}
	}

	ragReq := &pb.RAGRequest{
		Query:     req.Query,
		OrgId:     orgID,
		KbIds:     []string{kbID},
		SessionId: session.ID,
		Filters:   filters,
		Model:     req.Model,
		Provider:  req.Provider,
	}

	stream, err := s.grpcClient.Worker().QueryRAG(ctx, ragReq)
	if err != nil {
		return nil, apierror.NewInternal("failed to connect to AI worker: " + err.Error())
	}

	// Step 3: Stream tokens back via a channel.
	eventCh := make(chan SSEEvent, 32)

	go func() {
		defer close(eventCh)

		var fullText string
		var sources []model.ChatSource
		startTime := time.Now()

		for {
			chunk, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				select {
				case eventCh <- SSEEvent{Event: SSEEventError, Data: map[string]string{"error": err.Error()}}:
				case <-ctx.Done():
				}
				return
			}

			// Emit text token.
			if chunk.Text != "" {
				fullText += chunk.Text
				select {
				case eventCh <- SSEEvent{Event: SSEEventToken, Data: map[string]string{"text": chunk.Text}}:
				case <-ctx.Done():
					return
				}
			}

			// Collect sources from the chunk.
			if len(chunk.Sources) > 0 {
				var chunkSources []model.ChatSource
				for _, src := range chunk.Sources {
					chunkSources = append(chunkSources, model.ChatSource{
						DocumentID:   src.DocumentId,
						DocumentName: src.DocumentName,
						ChunkText:    src.ChunkText,
						Score:        src.Score,
					})
				}
				sources = append(sources, chunkSources...)

				select {
				case eventCh <- SSEEvent{Event: SSEEventSources, Data: map[string]any{"sources": chunkSources}}:
				case <-ctx.Done():
					return
				}
			}

			// If this is the final chunk, break out of the loop.
			if chunk.IsFinal {
				break
			}
		}

		// Step 4: Persist the assistant message with the full assembled text and token count.
		latencyMs := int(time.Since(startTime).Milliseconds())
		assistantTokens := model.EstimateTokens(fullText)
		var assistantMsg *model.ChatMessage

		saveErr := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var err error
			modelName := req.Model
			assistantMsg, err = s.chatRepo.CreateMessage(ctx, tx, &model.ChatMessage{
				SessionID:  session.ID,
				OrgID:      orgID,
				Role:       "assistant",
				Content:    fullText,
				TokenCount: &assistantTokens,
				ModelName:  strPtr(modelName),
				LatencyMs:  &latencyMs,
			})
			return err
		})

		if saveErr != nil {
			select {
			case eventCh <- SSEEvent{Event: SSEEventError, Data: map[string]string{"error": "failed to save assistant message"}}:
			case <-ctx.Done():
			}
			return
		}

		// Emit done event.
		msgID := ""
		if assistantMsg != nil {
			msgID = assistantMsg.ID
		}
		_ = userMsg // used above to ensure no unused-variable lint error
		_ = sources // sources already emitted per-chunk

		select {
		case eventCh <- SSEEvent{Event: SSEEventDone, Data: map[string]string{
			"session_id": session.ID,
			"message_id": msgID,
		}}:
		case <-ctx.Done():
		}
	}()

	return eventCh, nil
}

// BuildConversationContext assembles the sliding window history for a session.
// It fetches the last maxTurns*2 messages and trims to fit within the token budget.
func (s *ChatService) BuildConversationContext(ctx context.Context, orgID, sessionID string, maxTurns, tokenBudget int) (*model.ConversationContext, error) {
	var convCtx *model.ConversationContext
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		convCtx, err = s.buildConversationContextTx(ctx, tx, orgID, sessionID, maxTurns, tokenBudget)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to build conversation context: " + err.Error())
	}
	return convCtx, nil
}

// buildConversationContextTx is the transaction-scoped implementation of BuildConversationContext.
func (s *ChatService) buildConversationContextTx(ctx context.Context, tx pgx.Tx, orgID, sessionID string, maxTurns, tokenBudget int) (*model.ConversationContext, error) {
	if maxTurns <= 0 {
		maxTurns = model.DefaultMaxTurns
	}
	if tokenBudget <= 0 {
		tokenBudget = model.DefaultTokenBudget
	}

	// Fetch last maxTurns*2 messages (each turn = 1 user + 1 assistant).
	messages, err := s.chatRepo.ListRecentMessages(ctx, tx, sessionID, orgID, maxTurns*2)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return &model.ConversationContext{
			SessionID:   sessionID,
			Messages:    nil,
			TotalTokens: 0,
			Truncated:   false,
		}, nil
	}

	// Walk messages from newest to oldest, summing token counts.
	// We need to trim from the oldest end to fit within the token budget.
	totalTokens := 0
	truncated := false
	cutIndex := 0

	for i := len(messages) - 1; i >= 0; i-- {
		tokens := tokenCountOrEstimate(&messages[i])
		if totalTokens+tokens > tokenBudget {
			cutIndex = i + 1
			truncated = true
			break
		}
		totalTokens += tokens
	}

	// Return messages in chronological order from the cut point.
	result := messages[cutIndex:]

	return &model.ConversationContext{
		SessionID:   sessionID,
		Messages:    result,
		TotalTokens: totalTokens,
		Truncated:   truncated,
	}, nil
}

// tokenCountOrEstimate returns the token count from the message if available,
// otherwise estimates it as len(content) / 4.
func tokenCountOrEstimate(msg *model.ChatMessage) int {
	if msg.TokenCount != nil {
		return *msg.TokenCount
	}
	return model.EstimateTokens(msg.Content)
}

// GetHistory returns paginated message history for a session.
func (s *ChatService) GetHistory(ctx context.Context, orgID, sessionID string, limit, offset int) (*model.HistoryResponse, error) {
	var messages []model.ChatMessage
	var totalCount int

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify session exists.
		_, err := s.chatRepo.GetSession(ctx, tx, sessionID, orgID)
		if err != nil {
			return fmt.Errorf("session lookup: %w", err)
		}

		totalCount, err = s.chatRepo.CountSessionMessages(ctx, tx, sessionID, orgID)
		if err != nil {
			return err
		}

		messages, err = s.chatRepo.ListMessagesPaged(ctx, tx, sessionID, orgID, limit, offset)
		return err
	})
	if err != nil {
		if isNotFound(err) {
			return nil, apierror.NewNotFound("session not found")
		}
		return nil, apierror.NewInternal("failed to get history: " + err.Error())
	}

	if messages == nil {
		messages = []model.ChatMessage{}
	}

	return &model.HistoryResponse{
		SessionID:  sessionID,
		Messages:   messages,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// ListSessions returns active sessions for a knowledge base.
func (s *ChatService) ListSessions(ctx context.Context, orgID, kbID string, limit, offset int) (*model.SessionListResponse, error) {
	var sessions []model.ChatSession

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		sessions, err = s.chatRepo.ListSessions(ctx, tx, orgID, kbID, limit, offset)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list sessions: " + err.Error())
	}

	if sessions == nil {
		sessions = []model.ChatSession{}
	}

	return &model.SessionListResponse{
		Sessions: sessions,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

// DeleteSession removes a session and all its messages.
func (s *ChatService) DeleteSession(ctx context.Context, orgID, sessionID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.chatRepo.DeleteSession(ctx, tx, sessionID, orgID)
	})
	if err != nil {
		if isNotFound(err) {
			return apierror.NewNotFound("session not found")
		}
		return apierror.NewInternal("failed to delete session: " + err.Error())
	}
	return nil
}

// isNotFound checks if an error message indicates a not-found condition.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "session not found" ||
		msg == "no rows in result set" ||
		strings.HasSuffix(msg, "no rows in result set") ||
		strings.HasSuffix(msg, "session not found")
}

// strPtr returns a pointer to s, or nil if s is empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
