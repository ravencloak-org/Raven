package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func setupMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	genres := GenreListResponse{
		Genres: []Genre{
			{ID: 28, Name: "Action"},
			{ID: 35, Name: "Comedy"},
			{ID: 18, Name: "Drama"},
			{ID: 878, Name: "Science Fiction"},
			{ID: 16, Name: "Animation"},
		},
	}

	// Two movies per genre; movie 1 appears in both Action and Drama to test dedup.
	discoverResults := map[string]DiscoverResponse{
		"28": {Results: []MovieSummary{
			{ID: 1, Title: "Action Movie 1"},
			{ID: 2, Title: "Action Movie 2"},
		}},
		"35": {Results: []MovieSummary{
			{ID: 3, Title: "Comedy Movie 1"},
			{ID: 4, Title: "Comedy Movie 2"},
		}},
		"18": {Results: []MovieSummary{
			{ID: 1, Title: "Action Movie 1"}, // duplicate of Action genre
			{ID: 5, Title: "Drama Movie 1"},
		}},
		"878": {Results: []MovieSummary{
			{ID: 6, Title: "SciFi Movie 1"},
			{ID: 7, Title: "SciFi Movie 2"},
		}},
		"16": {Results: []MovieSummary{
			{ID: 8, Title: "Animation Movie 1"},
			{ID: 9, Title: "Animation Movie 2"},
		}},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/genre/movie/list", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") == "" {
			http.Error(w, "missing api_key", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(genres)
	})

	mux.HandleFunc("/discover/movie", func(w http.ResponseWriter, r *http.Request) {
		genreID := r.URL.Query().Get("with_genres")
		resp, ok := discoverResults[genreID]
		if !ok {
			http.Error(w, "unknown genre", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/movie/", func(w http.ResponseWriter, r *http.Request) {
		// Extract movie ID from path: /movie/{id}
		var movieID int
		_, err := fmt.Sscanf(r.URL.Path, "/movie/%d", &movieID)
		if err != nil {
			http.Error(w, "bad movie id", http.StatusBadRequest)
			return
		}

		detail := MovieDetail{
			ID:    movieID,
			Title: fmt.Sprintf("Movie %d", movieID),
			Credits: Credits{
				Cast: []CastMember{{Name: "Actor A", Character: "Hero", Order: 0}},
				Crew: []CrewMember{{Name: "Director X", Job: "Director"}},
			},
			Reviews: Reviews{
				Results: []Review{{Author: "reviewer1", Content: "Great movie!"}},
			},
			Keywords: Keywords{
				Keywords: []Keyword{{Name: "exciting"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(detail)
	})

	return httptest.NewServer(mux)
}

func TestFetchTopByGenres_Small(t *testing.T) {
	server := setupMockServer(t)
	defer server.Close()

	client := NewClient(server.URL, "test-api-key", server.Client())
	movies, err := client.FetchTopByGenres(context.Background(), "small")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We have 9 unique movie IDs across all genres (ID 1 is duplicated).
	if len(movies) != 9 {
		t.Fatalf("expected 9 unique movies, got %d", len(movies))
	}

	// Verify dedup: collect all IDs and check uniqueness.
	seen := make(map[int]bool)
	for _, m := range movies {
		if seen[m.ID] {
			t.Errorf("duplicate movie ID %d", m.ID)
		}
		seen[m.ID] = true
	}

	// Verify detail fields are populated.
	sort.Slice(movies, func(i, j int) bool { return movies[i].ID < movies[j].ID })
	first := movies[0]
	if len(first.Credits.Cast) == 0 {
		t.Error("expected credits cast to be populated")
	}
	if len(first.Reviews.Results) == 0 {
		t.Error("expected reviews to be populated")
	}
	if len(first.Keywords.Keywords) == 0 {
		t.Error("expected keywords to be populated")
	}
}

func TestFetchTopByGenres_UnsupportedSize(t *testing.T) {
	client := NewClient("http://unused", "key", nil)

	for _, size := range []string{"medium", "large"} {
		_, err := client.FetchTopByGenres(context.Background(), size)
		if err == nil {
			t.Errorf("expected error for size %q, got nil", size)
		}
	}
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	client := NewClient("http://example.com", "key", nil)
	if client.httpClient == nil {
		t.Error("expected default http client when nil is passed")
	}
}
