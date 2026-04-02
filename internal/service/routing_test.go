package service

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func TestValidateRoutingRule_Static_Valid(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeStatic, strPtr("kb-123"), nil, nil, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateRoutingRule_Static_MissingTargetKB(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeStatic, nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for static mode without target_kb_id")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected code 400, got %d", appErr.Code)
	}
}

func TestValidateRoutingRule_Static_EmptyTargetKB(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeStatic, strPtr(""), nil, nil, nil)
	if err == nil {
		t.Error("expected error for static mode with empty target_kb_id")
	}
}

func TestValidateRoutingRule_ColumnBased_Valid(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeColumnBased, nil, strPtr("department"), map[string]string{"eng": "kb-1"}, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateRoutingRule_ColumnBased_MissingColumn(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeColumnBased, nil, nil, map[string]string{"eng": "kb-1"}, nil)
	if err == nil {
		t.Error("expected error for column_based mode without discriminator_column")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected code 400, got %d", appErr.Code)
	}
}

func TestValidateRoutingRule_ColumnBased_EmptyMappings(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeColumnBased, nil, strPtr("department"), map[string]string{}, nil)
	if err == nil {
		t.Error("expected error for column_based mode with empty column_mappings")
	}
}

func TestValidateRoutingRule_ColumnBased_NilMappings(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeColumnBased, nil, strPtr("department"), nil, nil)
	if err == nil {
		t.Error("expected error for column_based mode with nil column_mappings")
	}
}

func TestValidateRoutingRule_Auto_Valid(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeAuto, nil, nil, nil, strPtr("Classify this document"))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateRoutingRule_Auto_MissingPrompt(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeAuto, nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for auto mode without classification_prompt")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected code 400, got %d", appErr.Code)
	}
}

func TestValidateRoutingRule_Auto_EmptyPrompt(t *testing.T) {
	err := validateRoutingRule(model.RoutingModeAuto, nil, nil, nil, strPtr(""))
	if err == nil {
		t.Error("expected error for auto mode with empty classification_prompt")
	}
}

func TestValidateRoutingRule_InvalidMode(t *testing.T) {
	err := validateRoutingRule(model.RoutingMode("invalid"), nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for invalid routing mode")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected code 400, got %d", appErr.Code)
	}
}

func TestValidRoutingModes(t *testing.T) {
	valid := []model.RoutingMode{
		model.RoutingModeStatic,
		model.RoutingModeColumnBased,
		model.RoutingModeAuto,
	}
	for _, rm := range valid {
		if !validRoutingModes[rm] {
			t.Errorf("expected %q to be a valid routing mode", rm)
		}
	}
	if validRoutingModes["invalid_mode"] {
		t.Error("expected 'invalid_mode' to be invalid")
	}
}
