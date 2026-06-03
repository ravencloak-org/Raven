package jobs

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// extractTextByMIME returns the human-readable text body of an uploaded
// document based on its MIME type. Text-based formats (markdown, plain,
// csv, html, json) are passed through as UTF-8 strings; binary formats
// (PDF for now) are parsed via a format-specific extractor.
//
// Unknown / unsupported MIMEs surface as an explicit error so the
// document-process handler can mark the row failed with a clear status
// instead of silently dropping the binary into a text column — which
// is how PDFs were 22021-ing the ChunkRepository.CreateChunk call.
func extractTextByMIME(mimeType string, raw []byte) (string, error) {
	// Strip parameters (charset, boundary, etc.) and lowercase the media
	// type so the switch matches "text/plain; charset=utf-8" the same
	// way it matches "text/plain".
	mt := normaliseMediaType(mimeType)
	switch mt {
	case "text/markdown", "text/plain", "text/csv", "text/html", "application/json":
		if !isProbablyUTF8(raw) {
			return "", fmt.Errorf("file declared as %s but contents are not valid UTF-8 — re-upload as the correct type", mimeType)
		}
		return string(raw), nil
	case "application/pdf":
		return extractPDFText(raw)
	}
	return "", fmt.Errorf("unsupported MIME type for text extraction: %s", mimeType)
}

// extractPDFText reads every page of the PDF and concatenates the plain
// text. ledongthuc/pdf doesn't preserve layout (no columns, no tables —
// it's a flat token stream), but it's pure Go, zero-CGO, and good enough
// for RAG indexing of typical document PDFs.
//
// Returns an error rather than silent empty text when extraction yields
// nothing useful — that way a scanned / image-only PDF surfaces as a
// clear failure to the user instead of getting silently marked ready
// with zero searchable content.
func extractPDFText(raw []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("parse PDF: %w", err)
	}
	var sb strings.Builder
	totalPages := r.NumPage()
	for pageIdx := 1; pageIdx <= totalPages; pageIdx++ {
		page := r.Page(pageIdx)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Per-page failures are usually layout edge cases; skip the
			// page rather than aborting the whole doc.
			continue
		}
		if text == "" {
			continue
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("PDF appears to contain no extractable text (scanned/image-only?)")
	}
	return out, nil
}

// normaliseMediaType strips RFC 2045 parameters from a MIME type string.
// We don't import mime.ParseMediaType here to avoid the allocation on
// every doc — a simple split is enough since we only need to match the
// well-known prefixes upstream.
func normaliseMediaType(mt string) string {
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = mt[:i]
	}
	return strings.ToLower(strings.TrimSpace(mt))
}

// isProbablyUTF8 returns true when the byte sequence is valid UTF-8.
// Used as a guard before inserting into a `text` column. unicode/utf8.Valid
// is the canonical stdlib check and correctly rejects overlong encodings,
// UTF-16 surrogate code points, and code points beyond U+10FFFF — none of
// which the hand-rolled byte-shape check above caught.
func isProbablyUTF8(b []byte) bool {
	return utf8.Valid(b)
}
