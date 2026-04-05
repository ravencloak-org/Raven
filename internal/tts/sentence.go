package tts

import (
	"regexp"
	"strings"
)

// sentenceRe splits text at sentence boundaries: period, exclamation mark,
// question mark, or ellipsis followed by whitespace or end-of-string.
// It keeps the punctuation attached to the preceding sentence.
var sentenceRe = regexp.MustCompile(`(?:[.!?]+["'\)\]]*|\.{3})(?:\s+|$)`)

// SplitSentences splits text into individual sentences at sentence boundaries.
// Each returned string is trimmed of leading/trailing whitespace. Empty strings
// are excluded. This is used for sentence-boundary dispatch so that each
// sentence can be synthesized independently for lower latency.
func SplitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	indices := sentenceRe.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return []string{text}
	}

	var sentences []string
	prev := 0
	for _, loc := range indices {
		end := loc[1]
		s := strings.TrimSpace(text[prev:end])
		if s != "" {
			sentences = append(sentences, s)
		}
		prev = end
	}
	// Capture any trailing text after the last sentence boundary.
	if prev < len(text) {
		s := strings.TrimSpace(text[prev:])
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	return sentences
}
