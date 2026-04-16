package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

// targetGenres are the genres we fetch top-rated movies for.
var targetGenres = []string{
	"Action",
	"Comedy",
	"Drama",
	"Science Fiction",
	"Animation",
}

// Client is a TMDB API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new TMDB client. If httpClient is nil, http.DefaultClient is used.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// FetchTopByGenres fetches top-rated movies for the target genres.
// size controls how many movies per genre: "small" = 10.
// "medium" and "large" are not yet implemented.
func (c *Client) FetchTopByGenres(ctx context.Context, size string) ([]MovieDetail, error) {
	if size != "small" {
		return nil, fmt.Errorf("tmdb: size %q not implemented", size)
	}

	genreMap, err := c.fetchGenres(ctx)
	if err != nil {
		return nil, fmt.Errorf("tmdb: fetch genres: %w", err)
	}

	// Collect unique movie summaries across target genres.
	seen := make(map[int]struct{})
	var summaries []MovieSummary

	for _, name := range targetGenres {
		genreID, ok := genreMap[name]
		if !ok {
			continue
		}
		discovered, discoverErr := c.discoverMovies(ctx, genreID)
		if discoverErr != nil {
			return nil, fmt.Errorf("tmdb: discover genre %q: %w", name, discoverErr)
		}
		for _, m := range discovered {
			if _, dup := seen[m.ID]; !dup {
				seen[m.ID] = struct{}{}
				summaries = append(summaries, m)
			}
		}
	}

	// Fetch details concurrently with a semaphore.
	const maxConcurrency = 10
	sem := make(chan struct{}, maxConcurrency)

	details := make([]MovieDetail, len(summaries))
	var mu sync.Mutex
	var firstErr error

	var wg sync.WaitGroup
	for i, s := range summaries {
		wg.Add(1)
		go func(idx int, movieID int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			detail, fetchErr := c.fetchMovieDetail(ctx, movieID)
			if fetchErr != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("tmdb: fetch movie %d: %w", movieID, fetchErr)
				}
				mu.Unlock()
				return
			}
			details[idx] = detail
		}(i, s.ID)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return details, nil
}

// fetchGenres retrieves the genre list and returns a name->ID mapping.
func (c *Client) fetchGenres(ctx context.Context) (map[string]int, error) {
	url := c.baseURL + "/genre/movie/list?api_key=" + c.apiKey

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var glr GenreListResponse
	if err := json.NewDecoder(resp.Body).Decode(&glr); err != nil {
		return nil, err
	}

	m := make(map[string]int, len(glr.Genres))
	for _, g := range glr.Genres {
		m[g.Name] = g.ID
	}
	return m, nil
}

// discoverMovies fetches top-rated movies for a specific genre.
func (c *Client) discoverMovies(ctx context.Context, genreID int) ([]MovieSummary, error) {
	url := c.baseURL + "/discover/movie?api_key=" + c.apiKey +
		"&sort_by=vote_average.desc&with_genres=" + strconv.Itoa(genreID) +
		"&vote_count.gte=1000"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var dr DiscoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, err
	}
	return dr.Results, nil
}

// fetchMovieDetail retrieves a single movie with credits, reviews, and keywords appended.
func (c *Client) fetchMovieDetail(ctx context.Context, movieID int) (MovieDetail, error) {
	url := fmt.Sprintf("%s/movie/%d?api_key=%s&append_to_response=credits,reviews,keywords",
		c.baseURL, movieID, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return MovieDetail{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MovieDetail{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return MovieDetail{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var md MovieDetail
	if err := json.NewDecoder(resp.Body).Decode(&md); err != nil {
		return MovieDetail{}, err
	}
	return md, nil
}
