package email

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSummaryHTMLIncludesBulletsAndCTA(t *testing.T) {
	html := RenderSummaryHTML(SummaryTemplateData{
		Greeting:       "Hi Jobin,",
		KBName:         "Acme Support",
		ChannelLabel:   "voice call",
		SessionDate:    time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC),
		Bullets:        []string{"You asked about pricing.", "We covered refunds."},
		ContinueURL:    "https://app.ravencloak.io/knowledge-bases/kb1?session=s1",
		UnsubscribeURL: "https://api.ravencloak.io/api/v1/notifications/unsubscribe?token=abc",
		SupportAddress: "support@ravencloak.io",
	})
	for _, frag := range []string{
		"Hi Jobin,",
		"Acme Support",
		"voice call",
		"You asked about pricing.",
		"We covered refunds.",
		"Continue the conversation",
		"Unsubscribe",
	} {
		if !strings.Contains(html, frag) {
			t.Errorf("expected html to contain %q", frag)
		}
	}
	if strings.Contains(html, "<style>") {
		t.Error("style block must not appear in email HTML (clients strip it)")
	}
}

func TestRenderSummaryTextIsPlainAscii(t *testing.T) {
	txt := RenderSummaryText(SummaryTemplateData{
		ChannelLabel:   "chat session",
		KBName:         "KB",
		SessionDate:    time.Now().UTC(),
		Bullets:        []string{"point one"},
		ContinueURL:    "https://x",
		UnsubscribeURL: "https://y",
		SupportAddress: "a@b.c",
	})
	if !strings.Contains(txt, "point one") {
		t.Fatal("bullet missing")
	}
	if strings.Contains(txt, "<") || strings.Contains(txt, ">") {
		t.Error("plain text should not contain HTML")
	}
}

func TestHTMLEscapesUserProvidedContent(t *testing.T) {
	html := RenderSummaryHTML(SummaryTemplateData{
		KBName:  "<script>alert('x')</script>",
		Bullets: []string{"<b>bad</b>"},
	})
	if strings.Contains(html, "<script>") {
		t.Error("unescaped script tag in rendered html")
	}
	if strings.Contains(html, "<b>bad</b>") {
		t.Error("bullet HTML not escaped")
	}
}
