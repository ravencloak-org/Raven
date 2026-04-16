package tmdb

import (
	"fmt"
	"strings"
)

const (
	maxCastMembers  = 5
	maxReviews      = 3
	maxReviewLength = 500
)

// RenderMarkdown converts a MovieDetail into a formatted Markdown string suitable for RAG ingestion.
func RenderMarkdown(movie MovieDetail) string {
	var sb strings.Builder

	// Title with year extracted from first 4 chars of ReleaseDate.
	year := ""
	if len(movie.ReleaseDate) >= 4 {
		year = movie.ReleaseDate[:4]
	}
	fmt.Fprintf(&sb, "# %s (%s)\n", movie.Title, year)

	// Overview section.
	fmt.Fprintf(&sb, "\n## Overview\n%s\n", movie.Overview)

	// Details section.
	fmt.Fprint(&sb, "\n## Details\n")

	// Genres.
	if len(movie.Genres) > 0 {
		names := make([]string, len(movie.Genres))
		for i, g := range movie.Genres {
			names[i] = g.Name
		}
		fmt.Fprintf(&sb, "- **Genres:** %s\n", strings.Join(names, ", "))
	}

	// Director — skip if no crew member has Job == "Director".
	for _, crew := range movie.Credits.Crew {
		if crew.Job == "Director" {
			fmt.Fprintf(&sb, "- **Director:** %s\n", crew.Name)
			break
		}
	}

	// Cast — top 5 members by order.
	if len(movie.Credits.Cast) > 0 {
		limit := len(movie.Credits.Cast)
		if limit > maxCastMembers {
			limit = maxCastMembers
		}
		names := make([]string, limit)
		for i := 0; i < limit; i++ {
			names[i] = movie.Credits.Cast[i].Name
		}
		fmt.Fprintf(&sb, "- **Cast:** %s\n", strings.Join(names, ", "))
	}

	// Rating.
	fmt.Fprintf(&sb, "- **Rating:** %.1f/10 (%d votes)\n", movie.VoteAverage, movie.VoteCount)

	// Runtime.
	fmt.Fprintf(&sb, "- **Runtime:** %d min\n", movie.Runtime)

	// Keywords.
	if len(movie.Keywords.Keywords) > 0 {
		names := make([]string, len(movie.Keywords.Keywords))
		for i, kw := range movie.Keywords.Keywords {
			names[i] = kw.Name
		}
		fmt.Fprintf(&sb, "- **Keywords:** %s\n", strings.Join(names, ", "))
	}

	// Reviews section — skip entirely if no reviews.
	if len(movie.Reviews.Results) > 0 {
		fmt.Fprint(&sb, "\n## Reviews\n")
		limit := len(movie.Reviews.Results)
		if limit > maxReviews {
			limit = maxReviews
		}
		for _, r := range movie.Reviews.Results[:limit] {
			content := r.Content
			if len(content) > maxReviewLength {
				content = content[:maxReviewLength] + "..."
			}
			fmt.Fprintf(&sb, "> \"%s\" — %s\n\n", content, r.Author)
		}
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}
