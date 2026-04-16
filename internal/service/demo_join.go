package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/ravencloak-org/Raven/internal/model"
)

// DemoOrgJoiner auto-joins new users to the demo organisation as viewers.
// It implements handler.DemoOrgJoiner.
type DemoOrgJoiner struct {
	orgSvc *OrgService
	wsSvc  *WorkspaceService
}

// NewDemoOrgJoiner creates a new DemoOrgJoiner.
func NewDemoOrgJoiner(orgSvc *OrgService, wsSvc *WorkspaceService) *DemoOrgJoiner {
	return &DemoOrgJoiner{orgSvc: orgSvc, wsSvc: wsSvc}
}

// JoinDemoOrg adds the user as a viewer to the first workspace of the "raven-demo" org.
// Best-effort: errors are logged but never propagated.
func (d *DemoOrgJoiner) JoinDemoOrg(ctx context.Context, userID string) {
	// Look up demo org by slug.
	demoOrg, err := d.orgSvc.GetBySlug(ctx, "raven-demo")
	if err != nil {
		slog.WarnContext(ctx, "demo-join: failed to look up demo org", "error", err)
		return
	}
	if demoOrg == nil {
		return // Demo org not seeded yet.
	}

	// List workspaces in demo org.
	workspaces, err := d.wsSvc.ListByOrg(ctx, demoOrg.ID)
	if err != nil || len(workspaces) == 0 {
		slog.WarnContext(ctx, "demo-join: no workspaces in demo org", "org_id", demoOrg.ID, "error", err)
		return
	}

	// Add user as viewer to the first workspace.
	ws := workspaces[0]
	_, err = d.wsSvc.AddMember(ctx, demoOrg.ID, ws.ID, model.AddWorkspaceMemberRequest{
		UserID: userID,
		Role:   "viewer",
	})
	if err != nil {
		// Duplicate member is fine — user already joined.
		if strings.Contains(err.Error(), "already a member") {
			return
		}
		slog.WarnContext(ctx, "demo-join: failed to add user to demo workspace",
			"user_id", userID, "ws_id", ws.ID, "error", err)
		return
	}

	slog.InfoContext(ctx, "demo-join: user auto-joined demo org",
		"user_id", userID, "org_id", demoOrg.ID, "ws_id", ws.ID)
}
