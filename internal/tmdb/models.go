package tmdb

// Genre represents a TMDB movie genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GenreListResponse is the response from /genre/movie/list.
type GenreListResponse struct {
	Genres []Genre `json:"genres"`
}

// MovieSummary is a compact movie record returned in discovery/search results.
type MovieSummary struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

// DiscoverResponse is the response from /discover/movie.
type DiscoverResponse struct {
	Results []MovieSummary `json:"results"`
}

// CastMember represents an actor in a movie's credits.
type CastMember struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

// CrewMember represents a crew member in a movie's credits.
type CrewMember struct {
	Name string `json:"name"`
	Job  string `json:"job"`
}

// Credits holds the cast and crew for a movie.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// Review represents a single user review.
type Review struct {
	Author  string `json:"author"`
	Content string `json:"content"`
}

// Reviews is the response shape for appended reviews.
type Reviews struct {
	Results []Review `json:"results"`
}

// Keyword represents a movie keyword/tag.
type Keyword struct {
	Name string `json:"name"`
}

// Keywords is the response shape for appended keywords.
type Keywords struct {
	Keywords []Keyword `json:"keywords"`
}

// MovieDetail is the full movie record returned by /movie/{id}
// with credits, reviews, and keywords appended.
type MovieDetail struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Overview    string   `json:"overview"`
	ReleaseDate string   `json:"release_date"`
	VoteAverage float64  `json:"vote_average"`
	VoteCount   int      `json:"vote_count"`
	Runtime     int      `json:"runtime"`
	Genres      []Genre  `json:"genres"`
	Credits     Credits  `json:"credits"`
	Reviews     Reviews  `json:"reviews"`
	Keywords    Keywords `json:"keywords"`
}
