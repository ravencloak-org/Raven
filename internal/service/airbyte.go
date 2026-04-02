package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// AirbyteService contains business logic for Airbyte connector management.
type AirbyteService struct {
	repo        *repository.AirbyteRepository
	pool        *pgxpool.Pool
	queueClient *queue.Client
}

// NewAirbyteService creates a new AirbyteService.
func NewAirbyteService(repo *repository.AirbyteRepository, pool *pgxpool.Pool, queueClient *queue.Client) *AirbyteService {
	return &AirbyteService{repo: repo, pool: pool, queueClient: queueClient}
}

// Create validates and persists a new Airbyte connector.
func (s *AirbyteService) Create(ctx context.Context, orgID, userID string, req model.CreateConnectorRequest) (*model.ConnectorResponse, error) {
	if req.SyncMode != "" && !model.ValidSyncModes[req.SyncMode] {
		return nil, apierror.NewBadRequest("invalid sync_mode: " + string(req.SyncMode))
	}

	var connector *model.AirbyteConnector
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		connector, err = s.repo.Create(ctx, tx, orgID, req, userID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates") {
			return nil, apierror.NewBadRequest("knowledge base not found or invalid reference")
		}
		return nil, apierror.NewInternal("failed to create connector: " + err.Error())
	}
	return connector.ToResponse(), nil
}

// GetByID retrieves a connector by ID within an org.
func (s *AirbyteService) GetByID(ctx context.Context, orgID, connectorID string) (*model.ConnectorResponse, error) {
	var connector *model.AirbyteConnector
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		connector, err = s.repo.GetByID(ctx, tx, orgID, connectorID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("connector not found")
		}
		return nil, apierror.NewInternal("failed to fetch connector: " + err.Error())
	}
	return connector.ToResponse(), nil
}

// List returns a paginated list of connectors for an organisation.
func (s *AirbyteService) List(ctx context.Context, orgID string, page, pageSize int) (*model.ConnectorListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var connectors []model.AirbyteConnector
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		connectors, total, err = s.repo.List(ctx, tx, orgID, page, pageSize)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list connectors: " + err.Error())
	}

	responses := lo.Map(connectors, func(c model.AirbyteConnector, _ int) model.ConnectorResponse {
		return *c.ToResponse()
	})
	responses = lo.Ternary(responses == nil, []model.ConnectorResponse{}, responses)

	return &model.ConnectorListResponse{
		Data:     responses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update validates and applies partial updates to a connector.
func (s *AirbyteService) Update(ctx context.Context, orgID, connectorID string, req model.UpdateConnectorRequest) (*model.ConnectorResponse, error) {
	if req.SyncMode != nil && !model.ValidSyncModes[*req.SyncMode] {
		return nil, apierror.NewBadRequest("invalid sync_mode: " + string(*req.SyncMode))
	}
	if req.Status != nil && !model.ValidConnectorStatuses[*req.Status] {
		return nil, apierror.NewBadRequest("invalid status: " + string(*req.Status))
	}

	var connector *model.AirbyteConnector
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		connector, err = s.repo.Update(ctx, tx, orgID, connectorID, req)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("connector not found")
		}
		return nil, apierror.NewInternal("failed to update connector: " + err.Error())
	}
	return connector.ToResponse(), nil
}

// Delete permanently removes a connector.
func (s *AirbyteService) Delete(ctx context.Context, orgID, connectorID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, connectorID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("connector not found")
		}
		return apierror.NewInternal("failed to delete connector: " + err.Error())
	}
	return nil
}

// TriggerSync enqueues an async sync job for a connector.
func (s *AirbyteService) TriggerSync(ctx context.Context, orgID, connectorID string) error {
	// Verify the connector exists and is active.
	var connector *model.AirbyteConnector
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		connector, err = s.repo.GetByID(ctx, tx, orgID, connectorID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return apierror.NewNotFound("connector not found")
		}
		return apierror.NewInternal("failed to fetch connector: " + err.Error())
	}
	if connector.Status != model.ConnectorStatusActive {
		return apierror.NewBadRequest("connector is not active, current status: " + string(connector.Status))
	}

	return s.queueClient.EnqueueAirbyteSync(ctx, queue.AirbyteSyncPayload{
		ConnectorID:     connectorID,
		OrgID:           orgID,
		KnowledgeBaseID: connector.KnowledgeBaseID,
	})
}

// GetSyncHistory returns recent sync history for a connector.
func (s *AirbyteService) GetSyncHistory(ctx context.Context, orgID, connectorID string, limit int) ([]model.SyncHistoryResponse, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var history []model.SyncHistory
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		history, err = s.repo.ListSyncHistory(ctx, tx, connectorID, orgID, limit)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to fetch sync history: " + err.Error())
	}

	responses := lo.Map(history, func(h model.SyncHistory, _ int) model.SyncHistoryResponse {
		return *h.ToResponse()
	})
	responses = lo.Ternary(responses == nil, []model.SyncHistoryResponse{}, responses)

	return responses, nil
}
