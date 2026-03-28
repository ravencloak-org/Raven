package service

import (
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func TestValidateURL_Valid(t *testing.T) {
	cases := []string{
		"https://example.com",
		"http://example.com/path",
		"https://sub.example.com:8080/foo?bar=1",
	}
	for _, tc := range cases {
		if err := validateURL(tc); err != nil {
			t.Errorf("validateURL(%q) returned error: %v", tc, err)
		}
	}
}

func TestValidateURL_Invalid(t *testing.T) {
	cases := []struct {
		url  string
		desc string
	}{
		{"ftp://example.com", "non-http scheme"},
		{"not-a-url", "missing scheme"},
		{"://no-scheme", "empty scheme"},
		{"", "empty string"},
	}
	for _, tc := range cases {
		err := validateURL(tc.url)
		if err == nil {
			t.Errorf("validateURL(%q) [%s] expected error, got nil", tc.url, tc.desc)
			continue
		}
		appErr, ok := err.(*apierror.AppError)
		if !ok {
			t.Errorf("validateURL(%q) expected *apierror.AppError, got %T", tc.url, err)
			continue
		}
		if appErr.Code != 400 {
			t.Errorf("validateURL(%q) expected code 400, got %d", tc.url, appErr.Code)
		}
	}
}

func TestValidateCrawlDepth_Valid(t *testing.T) {
	for _, d := range []int{1, 2, 3, 4, 5} {
		depth := d
		if err := validateCrawlDepth(&depth); err != nil {
			t.Errorf("validateCrawlDepth(%d) returned error: %v", d, err)
		}
	}
	// nil is valid (uses default)
	if err := validateCrawlDepth(nil); err != nil {
		t.Errorf("validateCrawlDepth(nil) returned error: %v", err)
	}
}

func TestValidateCrawlDepth_Invalid(t *testing.T) {
	for _, d := range []int{0, -1, 6, 100} {
		depth := d
		err := validateCrawlDepth(&depth)
		if err == nil {
			t.Errorf("validateCrawlDepth(%d) expected error, got nil", d)
			continue
		}
		appErr, ok := err.(*apierror.AppError)
		if !ok {
			t.Errorf("validateCrawlDepth(%d) expected *apierror.AppError, got %T", d, err)
			continue
		}
		if appErr.Code != 400 {
			t.Errorf("validateCrawlDepth(%d) expected code 400, got %d", d, appErr.Code)
		}
	}
}

func TestValidSourceTypes(t *testing.T) {
	valid := []model.SourceType{
		model.SourceTypeWebPage,
		model.SourceTypeWebSite,
		model.SourceTypeSitemap,
		model.SourceTypeRSSFeed,
	}
	for _, st := range valid {
		if !validSourceTypes[st] {
			t.Errorf("expected %q to be a valid source type", st)
		}
	}
	if validSourceTypes["invalid_type"] {
		t.Error("expected 'invalid_type' to be invalid")
	}
}

func TestValidCrawlFrequencies(t *testing.T) {
	valid := []model.CrawlFrequency{
		model.CrawlFrequencyManual,
		model.CrawlFrequencyDaily,
		model.CrawlFrequencyWeekly,
		model.CrawlFrequencyMonthly,
	}
	for _, cf := range valid {
		if !validCrawlFrequencies[cf] {
			t.Errorf("expected %q to be a valid crawl frequency", cf)
		}
	}
	if validCrawlFrequencies["hourly"] {
		t.Error("expected 'hourly' to be invalid")
	}
}
