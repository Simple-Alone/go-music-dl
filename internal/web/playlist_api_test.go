package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func newPlaylistAPITestRouter(providers playlistAPIProviders) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerPlaylistAPIRoutes(router.Group(RoutePrefix), providers)
	return router
}

func requestPlaylistAPI(t *testing.T, router http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestPlaylistAPISourcesExposeActualCapabilities(t *testing.T) {
	providers := playlistAPIProviders{
		sourceNames:       func() []string { return []string{"alpha", "beta"} },
		sourceDescription: func(source string) string { return strings.ToUpper(source) },
		search: func(source string) core.SearchPlaylistFunc {
			if source == "alpha" {
				return func(string) ([]model.Playlist, error) { return nil, nil }
			}
			return nil
		},
		categories: func(source string) core.PlaylistCategoriesFunc {
			if source == "alpha" {
				return func() ([]model.PlaylistCategory, error) { return nil, nil }
			}
			return nil
		},
		category: func(source string) core.CategoryPlaylistsFunc {
			if source == "alpha" {
				return func(string, int, int) ([]model.Playlist, error) { return nil, nil }
			}
			return nil
		},
		recommend: func(source string) func() ([]model.Playlist, error) {
			if source == "beta" {
				return func() ([]model.Playlist, error) { return nil, nil }
			}
			return nil
		},
		userPlaylists: func(source string) core.UserPlaylistsFunc {
			if source == "beta" {
				return func(int, int) ([]model.Playlist, error) { return nil, nil }
			}
			return nil
		},
	}
	recorder := requestPlaylistAPI(t, newPlaylistAPITestRouter(providers), RoutePrefix+"/api/playlist/sources")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Sources []playlistAPIPlatform `json:"sources"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []playlistAPIPlatform{
		{ID: "alpha", Name: "ALPHA", Search: true, Categories: true},
		{ID: "beta", Name: "BETA", Recommend: true, UserPlaylists: true},
	}
	if !reflect.DeepEqual(payload.Sources, want) {
		t.Fatalf("sources = %#v, want %#v", payload.Sources, want)
	}
}

func TestPlaylistAPIRoutesMapPlatformResults(t *testing.T) {
	var categoryArgs struct {
		id       string
		page     int
		pageSize int
	}
	var userArgs struct {
		page  int
		limit int
	}
	var detailID string
	providers := playlistAPIProviders{
		sourceNames:       func() []string { return []string{"test"} },
		sourceDescription: func(string) string { return "Test Music" },
		search: func(string) core.SearchPlaylistFunc {
			return func(keyword string) ([]model.Playlist, error) {
				if keyword != "focus" {
					t.Fatalf("search keyword = %q, want focus", keyword)
				}
				return []model.Playlist{{ID: "search-1", Name: "Search Result", Description: "Found", Cover: "https://example.test/search.jpg", TrackCount: 12}}, nil
			}
		},
		categories: func(string) core.PlaylistCategoriesFunc {
			return func() ([]model.PlaylistCategory, error) {
				return []model.PlaylistCategory{{ID: "study", Name: "Study", Group: "Scene", Hot: true}}, nil
			}
		},
		category: func(string) core.CategoryPlaylistsFunc {
			return func(categoryID string, page, pageSize int) ([]model.Playlist, error) {
				categoryArgs.id, categoryArgs.page, categoryArgs.pageSize = categoryID, page, pageSize
				return []model.Playlist{{ID: "category-1", Name: "Category Result", Source: "test"}}, nil
			}
		},
		recommend: func(string) func() ([]model.Playlist, error) {
			return func() ([]model.Playlist, error) {
				return []model.Playlist{{ID: "recommend-1", Name: "Recommended"}}, nil
			}
		},
		userPlaylists: func(string) core.UserPlaylistsFunc {
			return func(page, limit int) ([]model.Playlist, error) {
				userArgs.page, userArgs.limit = page, limit
				return []model.Playlist{{ID: "user-1", Name: "Saved"}}, nil
			}
		},
		detail: func(string) func(string) ([]model.Song, error) {
			return func(playlistID string) ([]model.Song, error) {
				detailID = playlistID
				return []model.Song{{ID: "song-1", Name: "Song", Artist: "Artist", Album: "Album", Cover: "https://example.test/song.jpg", Duration: 180, Extra: map[string]string{"hash": "abc"}}}, nil
			}
		},
	}
	router := newPlaylistAPITestRouter(providers)

	categories := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/categories?source=test")
	if categories.Code != http.StatusOK || !strings.Contains(categories.Body.String(), `"id":"study"`) || !strings.Contains(categories.Body.String(), `"hot":true`) {
		t.Fatalf("unexpected categories response: status=%d body=%s", categories.Code, categories.Body.String())
	}
	search := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/search?source=test&q=focus")
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"track_count":12`) || !strings.Contains(search.Body.String(), `"source":"test"`) {
		t.Fatalf("unexpected search response: status=%d body=%s", search.Code, search.Body.String())
	}
	category := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/category?source=test&category_id=study&page=2&page_size=25")
	if category.Code != http.StatusOK || categoryArgs.id != "study" || categoryArgs.page != 2 || categoryArgs.pageSize != 25 {
		t.Fatalf("unexpected category response/args: status=%d args=%#v body=%s", category.Code, categoryArgs, category.Body.String())
	}
	recommend := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/recommend?source=test")
	if recommend.Code != http.StatusOK || !strings.Contains(recommend.Body.String(), `"id":"recommend-1"`) {
		t.Fatalf("unexpected recommend response: status=%d body=%s", recommend.Code, recommend.Body.String())
	}
	user := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/user?source=test&page=3&limit=20")
	if user.Code != http.StatusOK || userArgs.page != 3 || userArgs.limit != 20 {
		t.Fatalf("unexpected user response/args: status=%d args=%#v body=%s", user.Code, userArgs, user.Body.String())
	}
	songs := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/songs?source=test&id=playlist-1")
	if songs.Code != http.StatusOK || detailID != "playlist-1" || !strings.Contains(songs.Body.String(), `"hash":"abc"`) || !strings.Contains(songs.Body.String(), `"duration":180`) {
		t.Fatalf("unexpected songs response: status=%d id=%q body=%s", songs.Code, detailID, songs.Body.String())
	}
}

func TestPlaylistAPIRoutesDistinguishEmptyUnsupportedAndUpstreamFailure(t *testing.T) {
	providers := playlistAPIProviders{
		sourceNames: func() []string { return []string{"test"} },
		search: func(string) core.SearchPlaylistFunc {
			return func(keyword string) ([]model.Playlist, error) {
				if keyword == "fail" {
					return nil, errors.New("provider unavailable")
				}
				return []model.Playlist{}, nil
			}
		},
	}
	router := newPlaylistAPITestRouter(providers)

	empty := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/search?source=test&q=empty")
	if empty.Code != http.StatusOK || strings.TrimSpace(empty.Body.String()) != `{"playlists":[]}` {
		t.Fatalf("empty response = status %d body %s", empty.Code, empty.Body.String())
	}
	failure := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/search?source=test&q=fail")
	if failure.Code != http.StatusBadGateway || !strings.Contains(failure.Body.String(), `"error":"upstream_failed"`) {
		t.Fatalf("failure response = status %d body %s", failure.Code, failure.Body.String())
	}
	unsupported := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/categories?source=test")
	if unsupported.Code != http.StatusNotFound || !strings.Contains(unsupported.Body.String(), `"error":"unsupported_capability"`) {
		t.Fatalf("unsupported response = status %d body %s", unsupported.Code, unsupported.Body.String())
	}
	invalidSource := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/search?source=unknown&q=focus")
	if invalidSource.Code != http.StatusBadRequest || !strings.Contains(invalidSource.Body.String(), `"error":"invalid_source"`) {
		t.Fatalf("invalid source response = status %d body %s", invalidSource.Code, invalidSource.Body.String())
	}
	invalidPage := requestPlaylistAPI(t, router, RoutePrefix+"/api/playlist/user?source=test&page=0")
	if invalidPage.Code != http.StatusBadRequest || !strings.Contains(invalidPage.Body.String(), `"error":"invalid_pagination"`) {
		t.Fatalf("invalid pagination response = status %d body %s", invalidPage.Code, invalidPage.Body.String())
	}
}
