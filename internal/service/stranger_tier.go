package service

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StrangerTierChecker implements handler.OrgTierChecker by querying the
// subscriptions table to determine whether an organisation's active plan
// allows per-user stranger controls.
//
// Free-tier orgs (plan_id = "plan_free" or no active subscription) receive
// only global rate limiting. Pro and Enterprise orgs get per-user block/ban
// and rate-limit override capabilities.
type StrangerTierChecker struct {
	pool *pgxpool.Pool
}

// NewStrangerTierChecker creates a StrangerTierChecker backed by pool.
func NewStrangerTierChecker(pool *pgxpool.Pool) *StrangerTierChecker {
	return &StrangerTierChecker{pool: pool}
}

// IsPerUserControlsAllowed returns true when the org's active subscription is
// on a plan other than "plan_free". If no subscription row exists the org is
// treated as free-tier (returns false).
func (c *StrangerTierChecker) IsPerUserControlsAllowed(ctx context.Context, orgID string) (bool, error) {
	var planID string
	err := c.pool.QueryRow(ctx,
		`SELECT plan_id FROM subscriptions
		 WHERE org_id = $1
		   AND status IN ('active', 'trialing', 'past_due')
		 LIMIT 1`,
		orgID,
	).Scan(&planID)
	if err != nil {
		// pgx returns pgx.ErrNoRows when there is no subscription; treat as free.
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		slog.WarnContext(ctx, "StrangerTierChecker: subscription lookup failed",
			"org_id", orgID,
			"error", err.Error(),
		)
		return false, err
	}
	return planID != "plan_free", nil
}
