package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

// mockSecurityEvaluator implements middleware.SecurityEvaluator for tests.
type mockSecurityEvaluator struct {
	evaluateFn func(ctx context.Context, orgID, clientIP, path, method, userAgent string) (*middleware.SecurityRuleAction, error)
}

func (m *mockSecurityEvaluator) EvaluateRequest(ctx context.Context, orgID, clientIP, path, method, userAgent string) (*middleware.SecurityRuleAction, error) {
	return m.evaluateFn(ctx, orgID, clientIP, path, method, userAgent)
}

func newSecurityRulesRouter(evaluator middleware.SecurityEvaluator, setOrgID bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if setOrgID {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyOrgID), "org-123")
			c.Next()
		})
	}
	r.Use(middleware.SecurityRulesMiddleware(evaluator))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestSecurityRulesMiddleware_NoOrgID_Passes(t *testing.T) {
	evaluator := &mockSecurityEvaluator{
		evaluateFn: func(_ context.Context, _, _, _, _, _ string) (*middleware.SecurityRuleAction, error) {
			t.Fatal("evaluator should not be called when no org ID is set")
			return nil, nil
		},
	}
	r := newSecurityRulesRouter(evaluator, false)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSecurityRulesMiddleware_NilAction_Passes(t *testing.T) {
	evaluator := &mockSecurityEvaluator{
		evaluateFn: func(_ context.Context, _, _, _, _, _ string) (*middleware.SecurityRuleAction, error) {
			return nil, nil
		},
	}
	r := newSecurityRulesRouter(evaluator, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSecurityRulesMiddleware_Block_Returns403(t *testing.T) {
	evaluator := &mockSecurityEvaluator{
		evaluateFn: func(_ context.Context, _, _, _, _, _ string) (*middleware.SecurityRuleAction, error) {
			return &middleware.SecurityRuleAction{Block: true, RuleID: "rule-1"}, nil
		},
	}
	r := newSecurityRulesRouter(evaluator, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSecurityRulesMiddleware_NonBlockAction_Passes(t *testing.T) {
	evaluator := &mockSecurityEvaluator{
		evaluateFn: func(_ context.Context, _, _, _, _, _ string) (*middleware.SecurityRuleAction, error) {
			return &middleware.SecurityRuleAction{Block: false, Throttle: true}, nil
		},
	}
	r := newSecurityRulesRouter(evaluator, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSecurityRulesMiddleware_EvalError_FailsOpen(t *testing.T) {
	evaluator := &mockSecurityEvaluator{
		evaluateFn: func(_ context.Context, _, _, _, _, _ string) (*middleware.SecurityRuleAction, error) {
			return nil, errors.New("cache unavailable")
		},
	}
	r := newSecurityRulesRouter(evaluator, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	// Fail open: should still return 200
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (fail open), got %d", w.Code)
	}
}
