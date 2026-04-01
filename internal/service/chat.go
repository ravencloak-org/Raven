package service

import (
	"context"
	"fmt"
	"io"
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
// It creates or looks up a session, persists the user message, streams
// the gRPC response, and persists the assistant reply when complete.
func (s *ChatService) StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan SSEEvent, error) {
	var session *model.ChatSession
	var userMsg *model.ChatMessage

	// Step 1: Look up or create session, and save the user message.
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		if req.SessionID != "" {
			session, err = s.chatRepo.GetSession(ctx, tx, req.SessionID, orgID)
			if err != nil {
				return fmt.Errorf("session lookup: %w", err)
			}
		} else {
			session, err = s.chatRepo.CreateSession(ctx, tx, &model.ChatSession{
				OrgID:           orgID,
				KnowledgeBaseID: kbID,
				SessionToken:    uuid.New().String(),
				Metadata:        map[string]any{},
			})
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
		}

		// Save user message.
		userMsg, err = s.chatRepo.CreateMessage(ctx, tx, &model.ChatMessage{
			SessionID: session.ID,
			OrgID:     orgID,
			Role:      "user",
			Content:   req.Query,
		})
		if err != nil {
			return fmt.Errorf("save user message: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to initialise chat: " + err.Error())
	}

	// Step 2: Call the gRPC AI worker.
	ragReq := &pb.RAGRequest{
		Query:     req.Query,
		OrgId:     orgID,
		KbIds:     []string{kbID},
		SessionId: session.ID,
		Filters:   req.Filters,
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

		// Step 4: Persist the assistant message with the full assembled text.
		latencyMs := int(time.Since(startTime).Milliseconds())
		var assistantMsg *model.ChatMessage

		saveErr := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
			var err error
			modelName := req.Model
			assistantMsg, err = s.chatRepo.CreateMessage(ctx, tx, &model.ChatMessage{
				SessionID: session.ID,
				OrgID:     orgID,
				Role:      "assistant",
				Content:   fullText,
				ModelName: strPtr(modelName),
				LatencyMs: &latencyMs,
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

// strPtr returns a pointer to s, or nil if s is empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
