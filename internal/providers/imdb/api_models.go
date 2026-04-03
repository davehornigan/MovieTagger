package imdb

import "strings"

type omdbSearchResponse struct {
	Search   []omdbSearchItem `json:"Search"`
	Response string           `json:"Response"`
	Error    string           `json:"Error"`
}

func (r omdbSearchResponse) IsSuccess() bool {
	return strings.EqualFold(strings.TrimSpace(r.Response), "True")
}

type omdbSearchItem struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	IMDbID string `json:"imdbID"`
	Type   string `json:"Type"`
}

type omdbEpisodeResponse struct {
	Title    string `json:"Title"`
	Year     string `json:"Year"`
	IMDbID   string `json:"imdbID"`
	SeriesID string `json:"seriesID"`
	Response string `json:"Response"`
	Error    string `json:"Error"`
}

func (r omdbEpisodeResponse) IsSuccess() bool {
	return strings.EqualFold(strings.TrimSpace(r.Response), "True")
}
