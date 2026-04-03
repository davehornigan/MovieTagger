package providerfactory

import (
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestBuild_ProviderPriorityOrder(t *testing.T) {
	providers := Build(BuildOptions{
		IMDbAPIKey: "imdb-key",
		TMDbAPIKey: "tmdb-key",
	})
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].Kind() != model.ProviderIMDb {
		t.Fatalf("expected first provider imdb, got %s", providers[0].Kind())
	}
	if providers[1].Kind() != model.ProviderTMDb {
		t.Fatalf("expected second provider tmdb, got %s", providers[1].Kind())
	}
}
