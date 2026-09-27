package web

import (
	"errors"
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

func withGuessKugouProvider(t *testing.T, loggedIn bool, provider func(core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error)) {
	t.Helper()
	originalLoggedIn := guessKugouLoggedIn
	originalProvider := guessKugouPersonalProvider
	guessKugouLoggedIn = func() bool { return loggedIn }
	guessKugouPersonalProvider = provider
	t.Cleanup(func() {
		guessKugouLoggedIn = originalLoggedIn
		guessKugouPersonalProvider = originalProvider
	})
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
	withGuessKugouProvider(t, false, func(core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error) {
		t.Fatal("native provider must not be called while logged out")
		return core.KugouPersonalRecommendResult{}, nil
	})
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
	for _, want := range []string{"猜你喜欢", "本机播放记录推荐", "Alice", "Recommended", "换一批"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
}

func TestGuessYouLikeRouteUsesKugouNativeRecommendationWhenLoggedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotOptions core.KugouPersonalRecommendOptions
	withGuessKugouProvider(t, true, func(options core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error) {
		gotOptions = options
		return core.KugouPersonalRecommendResult{
			Songs: []model.Song{{
				ID:     "native-1",
				Name:   "Native Pick",
				Artist: "Kugou Artist",
				Source: "kugou",
				Extra:  map[string]string{"hash": "native-hash", "audio_id": "123"},
			}},
			Cursor: "99999998",
		}, nil
	})
	withGuessSearchProvider(t, func(string) core.SearchFunc {
		t.Fatal("history search must not be called in native mode")
		return nil
	})

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	registerGuessYouLikeRoute(router.Group(RoutePrefix))
	req := httptest.NewRequest("GET", RoutePrefix+"/guess_you_like?artists=Alice&refresh=1&hash=CURRENT&song_id=123&play_time=87&cursor=9988&remain_songcnt=2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotOptions.Action != "play" || gotOptions.Hash != "CURRENT" || gotOptions.SongID != "123" || gotOptions.PlayTime != 87 || gotOptions.Cursor != "9988" || gotOptions.RemainSongCount != 2 {
		t.Fatalf("unexpected native options: %#v", gotOptions)
	}
	body := rec.Body.String()
	for _, want := range []string{"酷狗原生推荐", "Native Pick", "Kugou Artist", `data-cursor="99999998"`, `data-kugou-fm-hash="native-hash"`, `data-kugou-fm-song-id="123"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
	if strings.Contains(body, "guess-seed\">Alice") {
		t.Fatalf("native response must not present local history seeds")
	}
}

func TestGuessYouLikeRouteShowsExplicitFallbackAfterNativeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withGuessKugouProvider(t, true, func(core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error) {
		return core.KugouPersonalRecommendResult{}, errors.New("upstream unavailable")
	})

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	registerGuessYouLikeRoute(router.Group(RoutePrefix))
	req := httptest.NewRequest("GET", RoutePrefix+"/guess_you_like?artists=Alice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"酷狗原生推荐加载失败", "upstream unavailable", "使用本机推荐", "useLocalGuessYouLike()"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
}

func TestGuessYouLikeRouteAllowsExplicitLocalModeWhileLoggedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withGuessKugouProvider(t, true, func(core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error) {
		t.Fatal("native provider must not be called in explicit local mode")
		return core.KugouPersonalRecommendResult{}, nil
	})
	withGuessSearchProvider(t, func(string) core.SearchFunc {
		return func(keyword string) ([]model.Song, error) {
			return []model.Song{{ID: "local-1", Name: "Local Pick", Artist: keyword}}, nil
		}
	})

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	registerGuessYouLikeRoute(router.Group(RoutePrefix))
	req := httptest.NewRequest("GET", RoutePrefix+"/guess_you_like?mode=local&artists=Alice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{"本机播放记录推荐", "Local Pick", "Alice"} {
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
		"function guessYouLikeURL(refresh = false, mode = \"\")",
		"readPlaybackHistory()",
		"function goToGuessYouLike()",
		"function refreshGuessYouLike(mode = \"\")",
		"function useLocalGuessYouLike()",
		"const KUGOU_FM_PREFETCH_THRESHOLD = 2",
		"function initializeKugouFMPage(root = document)",
		"async function loadMoreKugouFM(audio, remaining)",
		"function maybeExtendKugouFMQueue()",
		"function buildPlaybackAudioFromCard(card)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}
