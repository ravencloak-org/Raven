package account

import (
	"context"
	"time"

	"github.com/ravencloak-org/Raven/internal/mail"
)

// InactiveUser is a row returned by RetentionRepo.InactiveUsers.
type InactiveUser struct {
	ID         string
	Email      string
	LastActive time.Time
}

// RetentionRepo is the persistence surface for the retention purge.
// The repo decides what "inactive" means at the SQL level; the purger
// just applies the day-threshold policy on top.
type RetentionRepo interface {
	// InactiveUsers returns every user whose last activity is older than
	// `since`. Callers use this to find candidates for warn-or-delete.
	InactiveUsers(ctx context.Context, since time.Time) ([]InactiveUser, error)

	// MarkWarned records that a 7-day deletion warning was sent.
	MarkWarned(ctx context.Context, id string) error

	// HardDelete removes the user and cascades to their owned rows.
	HardDelete(ctx context.Context, id string) error
}

// RetentionPurger applies the 23-day-warn / 30-day-delete policy
// described in docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md
// §6. Invoked by the host-side systemd timer via the admin endpoint.
type RetentionPurger struct {
	Repo RetentionRepo
	Mail mail.Sender
}

// NewRetentionPurger returns a purger wired to repo + mailer. The mailer
// may be nil; in that case warning emails are skipped but the warning
// flag is still set in the repo.
func NewRetentionPurger(repo RetentionRepo, mailer mail.Sender) *RetentionPurger {
	return &RetentionPurger{Repo: repo, Mail: mailer}
}

// RunOnce applies the retention policy using `now` as the reference
// time. Anyone past the 30-day delete threshold is removed; anyone
// past the 23-day warning threshold (but not yet at 30) is warned.
func (p *RetentionPurger) RunOnce(ctx context.Context, now time.Time) error {
	warnCutoff := now.Add(-23 * 24 * time.Hour)
	deleteCutoff := now.Add(-30 * 24 * time.Hour)

	users, err := p.Repo.InactiveUsers(ctx, warnCutoff)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.LastActive.Before(deleteCutoff) {
			if err := p.Repo.HardDelete(ctx, u.ID); err != nil {
				return err
			}
			continue
		}
		if p.Mail != nil {
			_ = p.Mail.Send(ctx, mail.Message{
				To:      u.Email,
				Subject: "Your Raven demo account will be deleted in 7 days",
				Text: "You haven't used Raven for 23 days. " +
					"Inactive demo accounts are deleted at 30 days. " +
					"Sign in to keep your data: " +
					"https://demo.raven.ravencloak.org",
			})
		}
		if err := p.Repo.MarkWarned(ctx, u.ID); err != nil {
			return err
		}
	}
	return nil
}
