package tmdb

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		movie  MovieDetail
		checks func(t *testing.T, result string)
	}{
		{
			name: "full movie with all fields",
			movie: MovieDetail{
				ID:          550,
				Title:       "Inception",
				Overview:    "A thief who steals corporate secrets through dream-sharing technology.",
				ReleaseDate: "2010-07-16",
				VoteAverage: 8.4,
				VoteCount:   30000,
				Runtime:     148,
				Genres: []Genre{
					{ID: 28, Name: "Action"},
					{ID: 878, Name: "Sci-Fi"},
				},
				Credits: Credits{
					Cast: []CastMember{
						{Name: "Leonardo DiCaprio", Character: "Cobb", Order: 0},
						{Name: "Joseph Gordon-Levitt", Character: "Arthur", Order: 1},
						{Name: "Elliot Page", Character: "Ariadne", Order: 2},
						{Name: "Tom Hardy", Character: "Eames", Order: 3},
						{Name: "Ken Watanabe", Character: "Saito", Order: 4},
						{Name: "Cillian Murphy", Character: "Fischer", Order: 5},
					},
					Crew: []CrewMember{
						{Name: "Christopher Nolan", Job: "Director"},
						{Name: "Emma Thomas", Job: "Producer"},
					},
				},
				Reviews: Reviews{
					Results: []Review{
						{Author: "moviefan", Content: "A masterpiece of modern cinema."},
						{Author: "critic42", Content: "Visually stunning and intellectually engaging."},
					},
				},
				Keywords: Keywords{
					Keywords: []Keyword{
						{Name: "dream"},
						{Name: "heist"},
					},
				},
			},
			checks: func(t *testing.T, result string) {
				t.Helper()

				// Title with year
				if !strings.Contains(result, "# Inception (2010)") {
					t.Error("expected title with year '# Inception (2010)'")
				}

				// Overview section
				if !strings.Contains(result, "## Overview") {
					t.Error("expected '## Overview' section")
				}
				if !strings.Contains(result, "A thief who steals corporate secrets") {
					t.Error("expected overview text")
				}

				// Details section
				if !strings.Contains(result, "## Details") {
					t.Error("expected '## Details' section")
				}
				if !strings.Contains(result, "**Genres:** Action, Sci-Fi") {
					t.Error("expected genres line")
				}
				if !strings.Contains(result, "**Director:** Christopher Nolan") {
					t.Error("expected director line")
				}
				if !strings.Contains(result, "**Rating:** 8.4/10 (30000 votes)") {
					t.Error("expected rating line")
				}
				if !strings.Contains(result, "**Runtime:** 148 min") {
					t.Error("expected runtime line")
				}
				if !strings.Contains(result, "**Keywords:** dream, heist") {
					t.Error("expected keywords line")
				}

				// Cast: top 5 only (6th actor Cillian Murphy should NOT appear)
				if !strings.Contains(result, "Leonardo DiCaprio") {
					t.Error("expected first cast member")
				}
				if !strings.Contains(result, "Ken Watanabe") {
					t.Error("expected fifth cast member")
				}
				if strings.Contains(result, "Cillian Murphy") {
					t.Error("6th cast member should not appear (max 5)")
				}

				// Reviews section
				if !strings.Contains(result, "## Reviews") {
					t.Error("expected '## Reviews' section")
				}
				if !strings.Contains(result, `"A masterpiece of modern cinema."`) {
					t.Error("expected first review content")
				}
				if !strings.Contains(result, "— moviefan") {
					t.Error("expected first review author")
				}
				if !strings.Contains(result, "— critic42") {
					t.Error("expected second review author")
				}
			},
		},
		{
			name: "movie with no reviews",
			movie: MovieDetail{
				ID:          999,
				Title:       "No Reviews Film",
				Overview:    "A film nobody reviewed.",
				ReleaseDate: "2023-01-15",
				VoteAverage: 6.0,
				VoteCount:   100,
				Runtime:     90,
				Genres:      []Genre{{ID: 1, Name: "Drama"}},
				Credits: Credits{
					Cast: []CastMember{{Name: "Actor One", Character: "Role", Order: 0}},
					Crew: []CrewMember{{Name: "Some Director", Job: "Director"}},
				},
				Reviews:  Reviews{Results: []Review{}},
				Keywords: Keywords{Keywords: []Keyword{{Name: "indie"}}},
			},
			checks: func(t *testing.T, result string) {
				t.Helper()

				if !strings.Contains(result, "# No Reviews Film (2023)") {
					t.Error("expected title with year")
				}
				if strings.Contains(result, "## Reviews") {
					t.Error("Reviews section should be absent when there are no reviews")
				}
			},
		},
		{
			name: "movie with no crew (no director)",
			movie: MovieDetail{
				ID:          888,
				Title:       "No Crew Film",
				Overview:    "A film with no crew listed.",
				ReleaseDate: "2022-06-01",
				VoteAverage: 5.5,
				VoteCount:   50,
				Runtime:     120,
				Genres:      []Genre{{ID: 2, Name: "Comedy"}},
				Credits: Credits{
					Cast: []CastMember{{Name: "Solo Actor", Character: "Lead", Order: 0}},
					Crew: []CrewMember{},
				},
				Reviews: Reviews{
					Results: []Review{{Author: "viewer", Content: "Interesting film."}},
				},
				Keywords: Keywords{Keywords: []Keyword{{Name: "experimental"}}},
			},
			checks: func(t *testing.T, result string) {
				t.Helper()

				if !strings.Contains(result, "# No Crew Film (2022)") {
					t.Error("expected title with year")
				}
				if strings.Contains(result, "**Director:**") {
					t.Error("Director line should be absent when no crew member has Job=='Director'")
				}
				// Other sections should still be present
				if !strings.Contains(result, "## Details") {
					t.Error("expected Details section")
				}
				if !strings.Contains(result, "## Reviews") {
					t.Error("expected Reviews section (reviews exist)")
				}
			},
		},
		{
			name: "movie with long review truncated at 500 chars",
			movie: MovieDetail{
				ID:          777,
				Title:       "Long Review Film",
				Overview:    "A film with a very long review.",
				ReleaseDate: "2021-03-20",
				VoteAverage: 7.0,
				VoteCount:   200,
				Runtime:     110,
				Genres:      []Genre{{ID: 3, Name: "Thriller"}},
				Credits: Credits{
					Cast: []CastMember{{Name: "Star Actor", Character: "Hero", Order: 0}},
					Crew: []CrewMember{{Name: "Jane Doe", Job: "Director"}},
				},
				Reviews: Reviews{
					Results: []Review{
						{
							Author:  "verbose_reviewer",
							Content: strings.Repeat("A", 600),
						},
					},
				},
				Keywords: Keywords{Keywords: []Keyword{{Name: "suspense"}}},
			},
			checks: func(t *testing.T, result string) {
				t.Helper()

				if !strings.Contains(result, "# Long Review Film (2021)") {
					t.Error("expected title with year")
				}

				// The review should be truncated to 500 chars + "..."
				truncated := strings.Repeat("A", 500) + "..."
				if !strings.Contains(result, truncated) {
					t.Error("expected review content truncated to 500 chars with '...' suffix")
				}

				// Full 600-char content should NOT appear
				full := strings.Repeat("A", 600)
				if strings.Contains(result, full) {
					t.Error("full 600-char review should not appear (should be truncated)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderMarkdown(tt.movie)
			tt.checks(t, result)
		})
	}
}

func TestRenderMarkdown_MaxThreeReviews(t *testing.T) {
	movie := MovieDetail{
		ID:          111,
		Title:       "Many Reviews",
		Overview:    "Has many reviews.",
		ReleaseDate: "2020-01-01",
		VoteAverage: 7.5,
		VoteCount:   500,
		Runtime:     100,
		Genres:      []Genre{{ID: 1, Name: "Drama"}},
		Credits: Credits{
			Cast: []CastMember{{Name: "A", Character: "B", Order: 0}},
			Crew: []CrewMember{{Name: "Dir", Job: "Director"}},
		},
		Reviews: Reviews{
			Results: []Review{
				{Author: "r1", Content: "Review one."},
				{Author: "r2", Content: "Review two."},
				{Author: "r3", Content: "Review three."},
				{Author: "r4", Content: "Review four."},
			},
		},
		Keywords: Keywords{Keywords: []Keyword{{Name: "test"}}},
	}

	result := RenderMarkdown(movie)

	if !strings.Contains(result, "— r3") {
		t.Error("expected third review to be present")
	}
	if strings.Contains(result, "— r4") {
		t.Error("fourth review should not appear (max 3 reviews)")
	}
}
