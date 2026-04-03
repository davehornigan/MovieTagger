package tmdb

import (
	"strconv"
	"strings"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func mapMovieResults(items []movieItem) []model.SelectedMatchResult {
	out := make([]model.SelectedMatchResult, 0, len(items))
	for _, it := range items {
		title := strings.TrimSpace(it.OriginalTitle)
		if title == "" {
			title = strings.TrimSpace(it.Title)
		}
		if title == "" {
			continue
		}
		id := ""
		if it.ID > 0 {
			id = strconv.Itoa(it.ID)
		}
		out = append(out, model.SelectedMatchResult{
			Provider:      model.ProviderTMDb,
			Kind:          model.MediaKindMovie,
			Title:         title,
			OriginalTitle: strings.TrimSpace(it.OriginalTitle),
			Year:          yearFromDate(it.ReleaseDate),
			IDs: model.ProviderTags{
				TMDbID: id,
			},
			ProviderReference: id,
		})
	}
	return out
}

func mapSeriesResults(items []tvItem) []model.SelectedMatchResult {
	out := make([]model.SelectedMatchResult, 0, len(items))
	for _, it := range items {
		title := strings.TrimSpace(it.OriginalName)
		if title == "" {
			title = strings.TrimSpace(it.Name)
		}
		if title == "" {
			continue
		}
		id := ""
		if it.ID > 0 {
			id = strconv.Itoa(it.ID)
		}
		out = append(out, model.SelectedMatchResult{
			Provider:      model.ProviderTMDb,
			Kind:          model.MediaKindSeries,
			Title:         title,
			OriginalTitle: strings.TrimSpace(it.OriginalName),
			Year:          yearFromDate(it.FirstAirDate),
			IDs: model.ProviderTags{
				TMDbID: id,
			},
			ProviderReference: id,
		})
	}
	return out
}

func mapMovieDetail(it movieDetailResponse) model.SelectedMatchResult {
	id := ""
	if it.ID > 0 {
		id = strconv.Itoa(it.ID)
	}
	title := strings.TrimSpace(it.OriginalTitle)
	if title == "" {
		title = strings.TrimSpace(it.Title)
	}
	return model.SelectedMatchResult{
		Provider:      model.ProviderTMDb,
		Kind:          model.MediaKindMovie,
		Title:         title,
		OriginalTitle: strings.TrimSpace(it.OriginalTitle),
		Year:          yearFromDate(it.ReleaseDate),
		IDs: model.ProviderTags{
			IMDbID: strings.TrimSpace(it.ExternalIDs.IMDbID),
			TMDbID: id,
		},
		ProviderReference: id,
	}
}

func mapTVDetail(it tvDetailResponse) model.SelectedMatchResult {
	id := ""
	if it.ID > 0 {
		id = strconv.Itoa(it.ID)
	}
	title := strings.TrimSpace(it.OriginalName)
	if title == "" {
		title = strings.TrimSpace(it.Name)
	}
	return model.SelectedMatchResult{
		Provider:      model.ProviderTMDb,
		Kind:          model.MediaKindSeries,
		Title:         title,
		OriginalTitle: strings.TrimSpace(it.OriginalName),
		Year:          yearFromDate(it.FirstAirDate),
		IDs: model.ProviderTags{
			IMDbID: strings.TrimSpace(it.ExternalIDs.IMDbID),
			TMDbID: id,
		},
		ProviderReference: id,
	}
}

func yearFromDate(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil || year < 1900 || year > 2099 {
		return 0
	}
	return year
}
