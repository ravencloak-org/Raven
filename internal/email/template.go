package email

import (
	"bytes"
	"fmt"
	"html"
	"strings"
	"time"
)

// SummaryTemplateData is the data bound to the summary email template.
type SummaryTemplateData struct {
	Greeting        string    // e.g. "Hi Jobin," — empty falls back to "Hi,"
	KBName          string    // e.g. "Acme Support"
	ChannelLabel    string    // e.g. "voice call" | "chat session"
	SessionDate     time.Time // UTC
	Bullets         []string  // 3–5 second-person takeaways
	ContinueURL     string    // deep link back to the KB with UTM params
	UnsubscribeURL  string    // one-click unsubscribe endpoint on the API
	SupportAddress  string    // reply-to / footer contact address
}

// RenderSummaryHTML returns a responsive, email-client-safe HTML body.
// All CSS is inlined on each element because Gmail, Outlook, and most
// mobile clients strip <style> blocks silently.
func RenderSummaryHTML(d SummaryTemplateData) string {
	greeting := d.Greeting
	if greeting == "" {
		greeting = "Hi,"
	}
	var bullets bytes.Buffer
	for _, b := range d.Bullets {
		fmt.Fprintf(&bullets,
			`<li style="margin:0 0 8px 0;line-height:1.5;color:#1f2933;">%s</li>`,
			html.EscapeString(b),
		)
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%[1]s — session summary</title></head>
<body style="margin:0;padding:0;background:#f4f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
  style="background:#f4f5f7;padding:24px 12px;">
  <tr><td align="center">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0"
      style="max-width:560px;background:#ffffff;border-radius:8px;padding:32px;box-shadow:0 1px 3px rgba(0,0,0,.06);">
      <tr><td>
        <div style="font-size:13px;color:#616e7c;letter-spacing:.08em;text-transform:uppercase;margin-bottom:8px;">RAVEN</div>
        <h1 style="margin:0 0 16px 0;font-size:22px;color:#102a43;line-height:1.3;">Your %[2]s recap</h1>
        <p style="margin:0 0 16px 0;font-size:15px;color:#334e68;line-height:1.5;">%[3]s</p>
        <p style="margin:0 0 20px 0;font-size:15px;color:#334e68;line-height:1.5;">
          Here’s what you and %[1]s covered on %[4]s:
        </p>
        <ul style="margin:0 0 28px 0;padding-left:20px;">%[5]s</ul>
        <p style="margin:0 0 24px 0;">
          <a href="%[6]s"
            style="display:inline-block;background:#2b6cb0;color:#ffffff;text-decoration:none;
                   padding:12px 20px;border-radius:6px;font-size:15px;font-weight:600;">
            Continue the conversation &rarr;
          </a>
        </p>
        <hr style="border:none;border-top:1px solid #e4e7eb;margin:24px 0;">
        <p style="margin:0;font-size:12px;color:#829ab1;line-height:1.5;">
          You received this because email summaries are enabled for your account.
          <a href="%[7]s" style="color:#2b6cb0;text-decoration:underline;">Unsubscribe</a>
          &middot; Questions? Reply to <a href="mailto:%[8]s" style="color:#2b6cb0;">%[8]s</a>
        </p>
      </td></tr>
    </table>
  </td></tr>
</table></body></html>`,
		html.EscapeString(d.KBName),
		html.EscapeString(d.ChannelLabel),
		html.EscapeString(greeting),
		d.SessionDate.UTC().Format("2 Jan 2006 at 15:04 UTC"),
		bullets.String(),
		html.EscapeString(d.ContinueURL),
		html.EscapeString(d.UnsubscribeURL),
		html.EscapeString(d.SupportAddress),
	)
}

// RenderSummaryText returns the plain-text fallback body. It mirrors the HTML
// content line-for-line so spam filters that diff the two parts are happy.
func RenderSummaryText(d SummaryTemplateData) string {
	greeting := d.Greeting
	if greeting == "" {
		greeting = "Hi,"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Raven — your %s recap\n\n", d.ChannelLabel)
	fmt.Fprintf(&sb, "%s\n\n", greeting)
	fmt.Fprintf(&sb, "Here's what you and %s covered on %s:\n\n",
		d.KBName, d.SessionDate.UTC().Format("2 Jan 2006 at 15:04 UTC"))
	for _, b := range d.Bullets {
		fmt.Fprintf(&sb, "  * %s\n", b)
	}
	fmt.Fprintf(&sb, "\nContinue the conversation: %s\n\n", d.ContinueURL)
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "Unsubscribe: %s\n", d.UnsubscribeURL)
	fmt.Fprintf(&sb, "Questions? Email %s\n", d.SupportAddress)
	return sb.String()
}
