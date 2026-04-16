package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitMarkdownByHeadings_MultipleH2(t *testing.T) {
	md := `# Movie: The Matrix

A classic sci-fi film.

## Overview

The Matrix is a 1999 science fiction action film.

## Cast

- Keanu Reeves
- Laurence Fishburne

## Reviews

Critics praised the film for its visual effects.
`
	sections := SplitMarkdownByHeadings(md)

	require.Len(t, sections, 4)

	// First section: preamble with # title
	assert.Equal(t, "", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "# Movie: The Matrix")
	assert.Contains(t, sections[0].Content, "A classic sci-fi film.")

	// Second section: Overview
	assert.Equal(t, "Overview", sections[1].Heading)
	assert.Contains(t, sections[1].Content, "## Overview")
	assert.Contains(t, sections[1].Content, "1999 science fiction")

	// Third section: Cast
	assert.Equal(t, "Cast", sections[2].Heading)
	assert.Contains(t, sections[2].Content, "Keanu Reeves")

	// Fourth section: Reviews
	assert.Equal(t, "Reviews", sections[3].Heading)
	assert.Contains(t, sections[3].Content, "visual effects")
}

func TestSplitMarkdownByHeadings_NoHeadings(t *testing.T) {
	md := `Just some plain text without any headings.

Second paragraph.`

	sections := SplitMarkdownByHeadings(md)

	require.Len(t, sections, 1)
	assert.Equal(t, "", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "Just some plain text")
}

func TestSplitMarkdownByHeadings_OnlyH1(t *testing.T) {
	md := `# Title

Some content under the title.`

	sections := SplitMarkdownByHeadings(md)

	require.Len(t, sections, 1)
	assert.Equal(t, "", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "# Title")
	assert.Contains(t, sections[0].Content, "Some content under the title.")
}

func TestSplitMarkdownByHeadings_EmptyContent(t *testing.T) {
	sections := SplitMarkdownByHeadings("")
	// An empty string still produces one section with empty content.
	require.Len(t, sections, 1)
	assert.Equal(t, "", sections[0].Content)
}

func TestSplitMarkdownByHeadings_ConsecutiveHeadings(t *testing.T) {
	md := `## First
## Second
Some content here.`

	sections := SplitMarkdownByHeadings(md)

	require.Len(t, sections, 2)
	assert.Equal(t, "First", sections[0].Heading)
	assert.Equal(t, "## First", sections[0].Content)

	assert.Equal(t, "Second", sections[1].Heading)
	assert.Contains(t, sections[1].Content, "## Second")
	assert.Contains(t, sections[1].Content, "Some content here.")
}

func TestSplitMarkdownByHeadings_H3NotSplit(t *testing.T) {
	md := `## Main Section

### Subsection

Content under subsection.`

	sections := SplitMarkdownByHeadings(md)

	// ### should NOT trigger a split — only ## does.
	require.Len(t, sections, 1)
	assert.Equal(t, "Main Section", sections[0].Heading)
	assert.Contains(t, sections[0].Content, "### Subsection")
	assert.Contains(t, sections[0].Content, "Content under subsection.")
}
