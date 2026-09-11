package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func withGuessSearchProvider(t *testing.T, provider func(string) core.SearchFunc) {
	t.Helper()
	original := guessSearchFuncProvider
	guessSearchFuncProvider = provider
	t.Cleanup(func() { guessSearchFuncProvider = original })
}

func TestNormalizeGuessArtistsDeduplicatesAndLimits(t *testing.T) {
	got := normalizeGuessArtists([]string{" Alice ", "alice", "", "Bob", "C", "D", "E", "F", "G"})
	want := []string{"Alice", "Bob", "C", "D", "E", "F"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("normalizeGuessArtists() = %v, want %v", got, want)
	}
}

func TestLoadGuessSongsDeduplicatesAndExcludesHistory(t *testing.T) {
	withGuessSearchProvider(t, func(source string) core.SearchFunc {
		if source != "kugou" {
			return nil
		}
		return func(keyword string) ([]model.Song, error) {
			return []model.Song{
				{ID: "heard", Name: keyword + " Old"},
				{ID: "shared", Name: "Shared"},
				{ID: keyword, Name: keyword + " New"},
			}, nil
		}
	})

	songs, err := loadGuessSongs([]string{"Alice", "Bob"}, map[string]struct{}{"heard": {}}, "test")
	if err != nil {
		t.Fatalf("loadGuessSongs() error = %v", err)
	}
	if len(songs) != 3 {
		t.Fatalf("loadGuessSongs() returned %d songs, want 3: %#v", len(songs), songs)
	}
	seen := make(map[string]bool)
	for _, song := range songs {
		if song.Source != "kugou" {
			t.Fatalf("song source = %q, want kugou", song.Source)
		}
		seen[song.ID] = true
	}
	if seen["heard"] || !seen["shared"] || !seen["Alice"] || !seen["Bob"] {
		t.Fatalf("unexpected result IDs: %v", seen)
	}
}

func TestGuessYouLikeRouteRendersHistoryState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withGuessSearchProvider(t, func(string) core.SearchFunc {
		return func(keyword string) ([]model.Song, error) {
			return []model.Song{{ID: "song-1", Name: "Recommended", Artist: keyword}}, nil
		}
	})

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	group := router.Group(RoutePrefix)
	registerGuessYouLikeRoute(group)

	req := httptest.NewRequest("GET", RoutePrefix+"/guess_you_like?artists=Alice&refresh=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"猜你喜欢", "Alice", "Recommended", "换一批"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
}

func TestGuessYouLikeClientUsesPlaybackHistory(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(content)
	for _, want := range []string{
		"function guessArtistSeeds(entries)",
		"function guessYouLikeURL(refresh = false)",
		"readPlaybackHistory()",
		"function goToGuessYouLike()",
		"function refreshGuessYouLike()",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}
