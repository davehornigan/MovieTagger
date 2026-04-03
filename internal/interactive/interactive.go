package interactive

import (
	"context"
	"errors"

	"github.com/davehornigan/MovieTagger/internal/model"
)

// Selector resolves ambiguous provider candidates and optional plan confirmation.
type Selector interface {
	SelectMatch(ctx context.Context, item model.ScanResultItem, candidates []model.SelectedMatchResult) (model.SelectedMatchResult, error)
	ConfirmPlan(ctx context.Context, plan model.RenamePlan) (bool, error)
}

// ErrSkipSelection indicates the user chose to skip the current ambiguous item.
var ErrSkipSelection = errors.New("interactive: skip selection")
