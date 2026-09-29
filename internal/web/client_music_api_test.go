package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func newClientMusicAPITestRouter(providers clientMusicAPIProviders) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerClientMusicAPIRoutes(router.Group(RoutePrefix), providers)
	return router
}

func requestClientMusicAPI(t *testing.T, router http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestClientMusicAPIRequiresKugouLogin(t *testing.T) {
	router := newClientMusicAPITestRouter(clientMusicAPIProviders{kugouLoggedIn: func() bool { return false }})
	recorder := requestClientMusicAPI(t, router, RoutePrefix+"/api/recommend/kugou/songs")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"error":"login_required"`) {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestClientMusicAPIRejectsOversizedPlaybackState(t *testing.T) {
	router := newClientMusicAPITestRouter(clientMusicAPIProviders{kugouLoggedIn: func() bool { return true }})
	recorder := requestClientMusicAPI(t, router, RoutePrefix+"/api/recommend/kugou/songs?cursor="+strings.Repeat("x", 241))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"error":"invalid_playback_state"`) {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestClientMusicAPIMapsKugouContinuation(t *testing.T) {
	var got core.KugouPersonalRecommendOptions
	router := newClientMusicAPITestRouter(clientMusicAPIProviders{
		kugouLoggedIn: func() bool { return true },
		kugouPersonal: func(options core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error) {
			got = options
			return core.KugouPersonalRecommendResult{
				Cursor: "next-cursor",
				Songs: []model.Song{
					{ID: "duplicate", Name: "Old", Source: "kugou"},
					{ID: "new-song", Name: "New", Artist: "Artist", Source: "kugou", Extra: map[string]string{"hash": "HASH", "audio_id": "88"}},
				},
			}, nil
		},
	})
	target := RoutePrefix + "/api/recommend/kugou/songs?action=play&hash=CURRENT&song_id=77&play_time=61&cursor=old-cursor&remain_songcnt=2&exclude=duplicate"
	recorder := requestClientMusicAPI(t, router, target)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got.Action != "play" || got.Hash != "CURRENT" || got.SongID != "77" || got.PlayTime != 61 || got.Cursor != "old-cursor" || got.RemainSongCount != 2 {
		t.Fatalf("unexpected continuation options: %#v", got)
	}
	var payload struct {
		OK     bool              `json:"ok"`
		Cursor string            `json:"cursor"`
		Songs  []playlistAPISong `json:"songs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.OK || payload.Cursor != "next-cursor" || len(payload.Songs) != 1 || payload.Songs[0].ID != "new-song" {
		t.Fatalf("unexpected response: %#v", payload)
	}
}

func TestClientMusicAPIResolvesOriginalAndFallbackSources(t *testing.T) {
	validateCalls := 0
	switchCalls := 0
	providers := clientMusicAPIProviders{
		validate: func(song *model.Song) bool {
			validateCalls++
			return song.ID == "playable"
		},
		switchSource: func(name, artist, current, target string, duration int) (*model.Song, float64, error) {
			switchCalls++
			if name != "Restricted" || artist != "Singer" || current != "kugou" || duration != 210 {
				t.Fatalf("unexpected switch args: %q %q %q %q %d", name, artist, current, target, duration)
			}
			return &model.Song{ID: "qq-song", Name: name, Artist: artist, Source: "qq", Duration: duration}, 0.99, nil
		},
	}
	router := newClientMusicAPITestRouter(providers)

	original := requestClientMusicAPI(t, router, RoutePrefix+"/api/playback/resolve?id=playable&source=kugou&name=Playable&artist=Singer")
	if original.Code != http.StatusOK || !strings.Contains(original.Body.String(), `"fallback":false`) || !strings.Contains(original.Body.String(), `"playback_source":"kugou"`) {
		t.Fatalf("unexpected original response: status=%d body=%s", original.Code, original.Body.String())
	}

	extra := url.QueryEscape(`{"privilege":"10"}`)
	fallback := requestClientMusicAPI(t, router, RoutePrefix+"/api/playback/resolve?id=restricted&source=kugou&name=Restricted&artist=Singer&duration=210&extra="+extra)
	if fallback.Code != http.StatusOK || !strings.Contains(fallback.Body.String(), `"fallback":true`) || !strings.Contains(fallback.Body.String(), `"fallback_reason":"restricted"`) || !strings.Contains(fallback.Body.String(), `"playback_source":"qq"`) {
		t.Fatalf("unexpected fallback response: status=%d body=%s", fallback.Code, fallback.Body.String())
	}
	if validateCalls != 2 || switchCalls != 1 {
		t.Fatalf("validate calls=%d switch calls=%d", validateCalls, switchCalls)
	}
}

func TestClientMusicAPIReturnsNoPlayableSource(t *testing.T) {
	router := newClientMusicAPITestRouter(clientMusicAPIProviders{
		validate: func(*model.Song) bool { return false },
		switchSource: func(string, string, string, string, int) (*model.Song, float64, error) {
			return nil, 0, errors.New("no playable match")
		},
	})
	recorder := requestClientMusicAPI(t, router, RoutePrefix+"/api/playback/resolve?id=missing&source=kugou&name=Missing")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"error":"no_playable_source"`) || !strings.Contains(recorder.Body.String(), `"fallback_reason":"unavailable"`) {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
