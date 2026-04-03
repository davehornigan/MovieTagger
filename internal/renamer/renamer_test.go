package renamer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/davehornigan/MovieTagger/internal/model"
)

func TestExecute_ActualRename(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "old.mp4")
	to := filepath.Join(root, "new.mp4")
	mustWriteFile(t, from)

	exec := New(nil)
	report, err := exec.Execute(context.Background(), model.RenamePlan{
		Operations: []model.RenameOperation{
			{Type: model.RenameOpPrimaryFile, FromPath: from, ToPath: to},
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Applied != 1 || report.Skipped != 0 || len(report.Failed) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	assertExists(t, to)
	assertNotExists(t, from)
}

func TestExecute_DryRunNoChanges(t *testing.T) {
	root := t.TempDir()
	from := filepath.Join(root, "old.mp4")
	to := filepath.Join(root, "new.mp4")
	mustWriteFile(t, from)

	exec := New(nil)
	report, err := exec.Execute(context.Background(), model.RenamePlan{
		DryRun: true,
		Operations: []model.RenameOperation{
			{Type: model.RenameOpPrimaryFile, FromPath: from, ToPath: to},
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Applied != 1 {
		t.Fatalf("expected dry-run applied count 1, got %+v", report)
	}
	assertExists(t, from)
	assertNotExists(t, to)
}

func TestExecute_CollisionsSkipped(t *testing.T) {
	root := t.TempDir()
	fromA := filepath.Join(root, "a.mp4")
	fromB := filepath.Join(root, "b.mp4")
	target := filepath.Join(root, "same.mp4")
	mustWriteFile(t, fromA)
	mustWriteFile(t, fromB)

	exec := New(nil)
	report, err := exec.Execute(context.Background(), model.RenamePlan{
		Operations: []model.RenameOperation{
			{Type: model.RenameOpPrimaryFile, FromPath: fromA, ToPath: target},
			{Type: model.RenameOpPrimaryFile, FromPath: fromB, ToPath: target},
		},
		Collisions: []model.RenameCollision{
			{TargetPath: target, SourcePaths: []string{fromA, fromB}},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error when collisions exist")
	}
	if report.Skipped != 2 {
		t.Fatalf("expected both operations skipped, got %+v", report)
	}
	assertExists(t, fromA)
	assertExists(t, fromB)
	assertNotExists(t, target)
}

func TestExecute_GroupedRelatedFileRenames(t *testing.T) {
	root := t.TempDir()
	videoFrom := filepath.Join(root, "Movie.mp4")
	subFrom := filepath.Join(root, "Movie.en.srt")
	posterFrom := filepath.Join(root, "Movie-cover.jpg")
	videoTo := filepath.Join(root, "The Movie (2000).mp4")
	subTo := filepath.Join(root, "The Movie (2000).en.srt")
	posterTo := filepath.Join(root, "The Movie (2000)-cover.jpg")

	mustWriteFile(t, videoFrom)
	mustWriteFile(t, subFrom)
	mustWriteFile(t, posterFrom)

	exec := New(nil)
	report, err := exec.Execute(context.Background(), model.RenamePlan{
		Operations: []model.RenameOperation{
			{Type: model.RenameOpPrimaryFile, FromPath: videoFrom, ToPath: videoTo},
			{Type: model.RenameOpRelatedFile, FromPath: subFrom, ToPath: subTo, RelatedTo: videoFrom},
			{Type: model.RenameOpRelatedFile, FromPath: posterFrom, ToPath: posterTo, RelatedTo: videoFrom},
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if report.Applied != 3 {
		t.Fatalf("expected 3 applied ops, got %+v", report)
	}
	assertExists(t, videoTo)
	assertExists(t, subTo)
	assertExists(t, posterTo)
	assertNotExists(t, videoFrom)
	assertNotExists(t, subFrom)
	assertNotExists(t, posterFrom)
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q to not exist, got err=%v", path, err)
	}
}
