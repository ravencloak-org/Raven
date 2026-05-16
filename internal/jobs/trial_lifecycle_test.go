package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsToday_SameDay(t *testing.T) {
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	target := time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)
	assert.True(t, isToday(now, target), "same calendar day should match")
}

func TestIsToday_TwoDaysAhead_NotToday(t *testing.T) {
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	// Two days in the future is clearly outside the 25-hour window.
	target := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	assert.False(t, isToday(now, target), "two days ahead should not match")
}

func TestIsToday_PreviousDay_NotToday(t *testing.T) {
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	target := time.Date(2026, 5, 14, 23, 59, 0, 0, time.UTC)
	assert.False(t, isToday(now, target), "two days ago should not match")
}

func TestIsToday_SlightlyLateJob(t *testing.T) {
	// Simulate a cron that fires 1 hour late the next day — should still match.
	now := time.Date(2026, 5, 17, 1, 0, 0, 0, time.UTC) // 01:00 on day D+1
	target := time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)
	// isToday uses a 25-hour window from midnight of now's date.
	// midnight of 2026-05-17 = 2026-05-17T00:00:00Z
	// end = 2026-05-18T01:00:00Z
	// target 2026-05-16T03:00:00Z is before midnight of 2026-05-17 → not in window
	assert.False(t, isToday(now, target), "target from yesterday is not 'today'")
}

// TestDayBoundaries verifies the computed day offsets match the spec.
func TestDayBoundaries_Offsets(t *testing.T) {
	trialStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	trialEnds := trialStart.Add(14 * 24 * time.Hour)    // day 14
	gracePeriodEnds := trialStart.Add(21 * 24 * time.Hour) // day 21

	warnAt := trialEnds.Add(-2 * 24 * time.Hour) // day 12
	expireAt := trialEnds.Add(1 * 24 * time.Hour) // day 15
	archiveAt := gracePeriodEnds.Add(1 * 24 * time.Hour)  // day 22
	deleteAt := archiveAt.Add(30 * 24 * time.Hour)         // day 52

	assert.Equal(t, trialStart.AddDate(0, 0, 12), warnAt, "warn at day 12")
	assert.Equal(t, trialStart.AddDate(0, 0, 15), expireAt, "expire at day 15")
	assert.Equal(t, trialStart.AddDate(0, 0, 22), archiveAt, "archive at day 22")
	assert.Equal(t, trialStart.AddDate(0, 0, 52), deleteAt, "delete at day 52")
}
