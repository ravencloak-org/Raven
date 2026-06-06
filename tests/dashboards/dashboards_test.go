// Package dashboards_test smoke-tests the OpenObserve dashboard JSON files
// under deploy/observability/dashboards/ to keep them syntactically valid and
// schema-shaped. The shape we assert here is the in-repo convention used by
// every existing dashboard (api-overview, alerts, chat-completions,
// voice-sessions, whatsapp, marketplace): a top-level title/description plus
// either a panels[] array (regular dashboards) or an alerts[] array
// (alerts.json). Each panel has id/title/type/query/description.
//
// This test catches the easy mistakes — invalid JSON, missing required
// fields, duplicate panel ids, empty queries — without locking the schema
// down so tightly that adding a new optional field (thresholds,
// additional_queries) requires a test change. Tighten if drift becomes a
// problem.
package dashboards_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dashboardsDir is the path to the JSON dashboards, relative to this test
// file. The working directory at test time is the package directory
// (tests/dashboards), so we walk up two levels to the repo root.
const dashboardsDir = "../../deploy/observability/dashboards"

// panel is the shared shape for entries in either dashboard.panels or
// alerts.alerts. We only require id/title/description to be present —
// type/query are optional because alert entries use "condition" instead of
// "query" and have no "type".
type panel struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Query       string `json:"query"`
	Condition   string `json:"condition"`
	Description string `json:"description"`
}

type dashboard struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Panels      []panel `json:"panels"`
	Alerts      []panel `json:"alerts"`
}

// TestDashboardJSONParses walks every *.json file in deploy/observability/
// dashboards/ and asserts that it parses as the in-repo dashboard schema.
// Fails on: invalid JSON, missing title, empty panels+alerts, duplicate
// panel ids, panels with no query/condition, or panels with empty
// description.
func TestDashboardJSONParses(t *testing.T) {
	entries, err := os.ReadDir(dashboardsDir)
	if err != nil {
		t.Fatalf("read dashboards dir %q: %v", dashboardsDir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files = append(files, e.Name())
	}
	if len(files) == 0 {
		t.Fatalf("no *.json files found in %q — expected at least api-overview.json", dashboardsDir)
	}

	for _, name := range files {
		name := name // capture for subtest closure
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dashboardsDir, name)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var d dashboard
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}

			if d.Title == "" {
				t.Errorf("%s: title is empty", name)
			}
			if d.Description == "" {
				t.Errorf("%s: description is empty", name)
			}

			entries := append([]panel{}, d.Panels...)
			entries = append(entries, d.Alerts...)
			if len(entries) == 0 {
				t.Fatalf("%s: neither panels[] nor alerts[] is populated", name)
			}

			// Panel ids must be unique within a dashboard so the alerting
			// UI / dashboard exporter can address them stably.
			seenIDs := make(map[int]struct{}, len(d.Panels))
			for _, p := range d.Panels {
				if _, dup := seenIDs[p.ID]; dup {
					t.Errorf("%s: duplicate panel id %d", name, p.ID)
				}
				seenIDs[p.ID] = struct{}{}

				if p.Title == "" {
					t.Errorf("%s: panel id=%d has empty title", name, p.ID)
				}
				if p.Description == "" {
					t.Errorf("%s: panel id=%d has empty description", name, p.ID)
				}
				if p.Query == "" {
					t.Errorf("%s: panel id=%d has empty query", name, p.ID)
				}
				if p.Type == "" {
					t.Errorf("%s: panel id=%d has empty type", name, p.ID)
				}
			}

			// Alerts have a name/condition shape instead of id/query.
			for i, a := range d.Alerts {
				if a.Name == "" {
					t.Errorf("%s: alert index=%d has empty name", name, i)
				}
				if a.Condition == "" {
					t.Errorf("%s: alert index=%d (%q) has empty condition", name, i, a.Name)
				}
				if a.Description == "" {
					t.Errorf("%s: alert index=%d (%q) has empty description", name, i, a.Name)
				}
			}
		})
	}
}

// TestMarketplaceDashboardTiles is a targeted check that the marketplace
// dashboard delivers the six tiles called for by issue #737 / plan §6 —
// catches an accidental panel deletion in a future refactor that the
// generic schema check above would miss.
func TestMarketplaceDashboardTiles(t *testing.T) {
	path := filepath.Join(dashboardsDir, "marketplace.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var d dashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	const wantTiles = 6
	if got := len(d.Panels); got != wantTiles {
		t.Fatalf("marketplace.json: want %d panels, got %d", wantTiles, got)
	}

	// Each title fragment must appear in exactly one panel — guards against
	// rename drift but stays loose enough to allow copy editing.
	wantTitleFragments := []string{
		"Reports Submitted",
		"Reports by Status",
		"Takedowns by Source",
		"Top-Strike Orgs",
		"Time-to-Resolution",
		"DMCA Notices Pending",
	}
	for _, frag := range wantTitleFragments {
		matches := 0
		for _, p := range d.Panels {
			if strings.Contains(p.Title, frag) {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("marketplace.json: want exactly 1 panel whose title contains %q, got %d", frag, matches)
		}
	}
}
