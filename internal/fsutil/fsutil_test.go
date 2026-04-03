package fsutil

import (
	"reflect"
	"testing"
)

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		isVid bool
	}{
		{name: "mkv", path: "movie.mkv", isVid: true},
		{name: "mp4 uppercase ext", path: "movie.MP4", isVid: true},
		{name: "avi", path: "/tmp/movie.avi", isVid: true},
		{name: "subtitle", path: "movie.srt", isVid: false},
		{name: "no ext", path: "movie", isVid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsVideoFile(tt.path)
			if got != tt.isVid {
				t.Fatalf("IsVideoFile(%q) = %v, want %v", tt.path, got, tt.isVid)
			}
		})
	}
}

func TestFindRelatedFiles(t *testing.T) {
	videoPath := "/media/Movie (2000).mkv"

	siblings := []PathEntry{
		{Path: "/media/Movie (2000).srt", IsDir: false},
		{Path: "/media/Movie (2000).en.srt", IsDir: false},
		{Path: "/media/Movie (2000)-cover.jpg", IsDir: false},
		{Path: "/media/Movie (2000) trailer.txt", IsDir: false},
		{Path: "/media/Other Movie.srt", IsDir: false},
		{Path: "/media/Movie (2000).mp4", IsDir: false},     // must be excluded (video)
		{Path: "/media/sub/Movie (2000).srt", IsDir: false}, // different dir
		{Path: "/media/Movie (2000)-extras", IsDir: true},   // directories standalone
	}

	got := FindRelatedFiles(videoPath, false, siblings)
	want := []string{
		"/media/Movie (2000).srt",
		"/media/Movie (2000).en.srt",
		"/media/Movie (2000)-cover.jpg",
		"/media/Movie (2000) trailer.txt",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("related mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFindRelatedFiles_DirectoryHasNoRelated(t *testing.T) {
	got := FindRelatedFiles("/media/Season 1", true, []PathEntry{
		{Path: "/media/Season 1/episode.srt", IsDir: false},
	})
	if len(got) != 0 {
		t.Fatalf("expected no related files for directory, got %v", got)
	}
}

func TestSanitizeTitleForFilesystem(t *testing.T) {
	in := `A/B\C:D*E?F"G<H>I|J`
	got := SanitizeTitleForFilesystem(in)
	want := "A B C D E F G H I J"
	if got != want {
		t.Fatalf("sanitize mismatch: got %q want %q", got, want)
	}
}

func TestSanitizeTitleForFilesystem_PreservesUnicodeAndCasing(t *testing.T) {
	in := `Мой Фильм: L'Amour | Directors Cut`
	got := SanitizeTitleForFilesystem(in)
	want := "Мой Фильм L'Amour Directors Cut"
	if got != want {
		t.Fatalf("unicode sanitize mismatch: got %q want %q", got, want)
	}
}
