package service_test

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TestValidSyncModes ensures all expected sync modes are in the valid set.
func TestValidSyncModes(t *testing.T) {
	expected := []model.SyncMode{
		model.SyncModeFullRefresh,
		model.SyncModeIncremental,
		model.SyncModeCDC,
	}
	for _, mode := range expected {
		if !model.ValidSyncModes[mode] {
			t.Errorf("expected sync mode %q to be valid", mode)
		}
	}
}

// TestInvalidSyncMode ensures unknown sync modes are rejected.
func TestInvalidSyncMode(t *testing.T) {
	if model.ValidSyncModes["unknown_mode"] {
		t.Error("expected unknown sync mode to be invalid")
	}
}

// TestValidConnectorStatuses ensures all expected connector statuses are in the valid set.
func TestValidConnectorStatuses(t *testing.T) {
	expected := []model.ConnectorStatus{
		model.ConnectorStatusActive,
		model.ConnectorStatusPaused,
		model.ConnectorStatusError,
		model.ConnectorStatusDeleted,
	}
	for _, status := range expected {
		if !model.ValidConnectorStatuses[status] {
			t.Errorf("expected connector status %q to be valid", status)
		}
	}
}

// TestInvalidConnectorStatus ensures unknown statuses are rejected.
func TestInvalidConnectorStatus(t *testing.T) {
	if model.ValidConnectorStatuses["bogus"] {
		t.Error("expected unknown connector status to be invalid")
	}
}

// TestConnectorToResponse verifies the ToResponse conversion omits config.
func TestConnectorToResponse(t *testing.T) {
	connector := &model.AirbyteConnector{
		ID:              "conn-1",
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		Name:            "Test",
		ConnectorType:   "source-postgres",
		Config:          map[string]any{"host": "localhost", "password": "secret"},
		SyncMode:        model.SyncModeFullRefresh,
		Status:          model.ConnectorStatusActive,
	}
	resp := connector.ToResponse()
	if resp.ID != "conn-1" {
		t.Errorf("expected ID conn-1, got %s", resp.ID)
	}
	if resp.Name != "Test" {
		t.Errorf("expected Name Test, got %s", resp.Name)
	}
	if resp.Status != model.ConnectorStatusActive {
		t.Errorf("expected status active, got %s", resp.Status)
	}
}

// TestSyncHistoryToResponse verifies the SyncHistory to response conversion.
func TestSyncHistoryToResponse(t *testing.T) {
	history := &model.SyncHistory{
		ID:            "sh-1",
		ConnectorID:   "conn-1",
		OrgID:         "org-1",
		Status:        "completed",
		RecordsSynced: 42,
		RecordsFailed: 1,
		BytesSynced:   1024,
		ErrorMessage:  "",
	}
	resp := history.ToResponse()
	if resp.ID != "sh-1" {
		t.Errorf("expected ID sh-1, got %s", resp.ID)
	}
	if resp.RecordsSynced != 42 {
		t.Errorf("expected 42 records synced, got %d", resp.RecordsSynced)
	}
	if resp.ConnectorID != "conn-1" {
		t.Errorf("expected connector_id conn-1, got %s", resp.ConnectorID)
	}
}
