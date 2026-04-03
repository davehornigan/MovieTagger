package tmdb

type searchMovieResponse struct {
	Results []movieItem `json:"results"`
}

type movieItem struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
}

type searchTVResponse struct {
	Results []tvItem `json:"results"`
}

type tvItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	FirstAirDate string `json:"first_air_date"`
}

type episodeDetailResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	AirDate     string `json:"air_date"`
	ExternalIDs struct {
		IMDbID string `json:"imdb_id"`
	} `json:"external_ids"`
}
