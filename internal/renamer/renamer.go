package renamer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
)

// OperationError captures a failed execution step.
type OperationError struct {
	Operation model.RenameOperation
	Err       error
}

// ExecutionReport contains executor results.
type ExecutionReport struct {
	Applied int
	Skipped int
	Failed  []OperationError
}

// Executor applies a planner-produced rename plan.
type Executor interface {
	Execute(ctx context.Context, plan model.RenamePlan) (ExecutionReport, error)
}

type Service struct {
	logger logging.Logger
}

func New(logger logging.Logger) *Service {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Service{logger: logger}
}

func (s *Service) Execute(ctx context.Context, plan model.RenamePlan) (ExecutionReport, error) {
	report := ExecutionReport{}

	s.logger.LogRenamePlan(plan)

	if plan.HasBlockingIssues() {
		for _, c := range plan.Collisions {
			s.logger.LogCollision(c.TargetPath, c.SourcePaths)
		}
		report.Skipped = len(plan.Operations)
		return report, fmt.Errorf("rename plan has blocking validation issues")
	}

	groups := groupOperations(plan.Operations)
	for _, g := range groups {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		default:
		}

		exists, err := anyTargetExists(g.ops)
		if err != nil {
			for _, op := range g.ops {
				report.Failed = append(report.Failed, OperationError{Operation: op, Err: err})
			}
			continue
		}
		if exists {
			for _, op := range g.ops {
				report.Skipped++
				s.logger.LogSkip(op.FromPath, "target already exists")
			}
			continue
		}

		if plan.DryRun {
			for _, op := range g.ops {
				report.Applied++
				s.logger.Infof("dry-run rename from=%q to=%q", op.FromPath, op.ToPath)
			}
			continue
		}

		// Execute group atomically in order; if one fails, skip remaining for coherence.
		groupFailed := false
		for idx, op := range g.ops {
			if groupFailed {
				report.Skipped++
				s.logger.LogSkip(op.FromPath, "group execution halted due to previous failure")
				continue
			}

			if err := os.Rename(op.FromPath, op.ToPath); err != nil {
				report.Failed = append(report.Failed, OperationError{Operation: op, Err: err})
				s.logger.Errorf("rename failed from=%q to=%q err=%v", op.FromPath, op.ToPath, err)
				groupFailed = true
				// Keep remaining related operations skipped to avoid half-applied groups.
				for j := idx + 1; j < len(g.ops); j++ {
					report.Skipped++
					s.logger.LogSkip(g.ops[j].FromPath, "group execution halted due to previous failure")
				}
				break
			}
			report.Applied++
			s.logger.Infof("renamed from=%q to=%q", op.FromPath, op.ToPath)
		}
	}

	if len(report.Failed) > 0 {
		return report, fmt.Errorf("rename execution failed for %d operation(s)", len(report.Failed))
	}
	return report, nil
}

type operationGroup struct {
	key string
	ops []model.RenameOperation
}

func groupOperations(ops []model.RenameOperation) []operationGroup {
	byKey := map[string]*operationGroup{}
	order := make([]string, 0, len(ops))
	for _, op := range ops {
		key := groupKey(op)
		g, ok := byKey[key]
		if !ok {
			order = append(order, key)
			g = &operationGroup{key: key}
			byKey[key] = g
		}
		g.ops = append(g.ops, op)
	}

	out := make([]operationGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out
}

func groupKey(op model.RenameOperation) string {
	if op.RelatedTo != "" {
		return op.RelatedTo
	}
	return op.FromPath
}

func anyTargetExists(ops []model.RenameOperation) (bool, error) {
	for _, op := range ops {
		_, err := os.Stat(op.ToPath)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return false, err
	}
	return false, nil
}

type noopLogger struct{}

func (noopLogger) Infof(string, ...any)                                    {}
func (noopLogger) Warnf(string, ...any)                                    {}
func (noopLogger) Errorf(string, ...any)                                   {}
func (noopLogger) LogScanStart(string)                                     {}
func (noopLogger) LogScanEnd(string, time.Duration, error)                 {}
func (noopLogger) LogProviderCall(model.ProviderKind, string)              {}
func (noopLogger) LogProviderRetry(model.ProviderKind, string, int, error) {}
func (noopLogger) LogMatch(string, model.SelectedMatchResult)              {}
func (noopLogger) LogRenamePlan(model.RenamePlan)                          {}
func (noopLogger) LogSkip(string, string)                                  {}
func (noopLogger) LogCollision(string, []string)                           {}
func (noopLogger) LogInvalidSeriesStructure(string, string)                {}
func (noopLogger) Close() error                                            { return nil }
