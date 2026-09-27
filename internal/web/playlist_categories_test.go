package web

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func withKugouDesktopPlaylistPageProvider(t *testing.T, provider func(string, int, int, int) (core.KugouDesktopPlaylistPage, error)) {
	t.Helper()
	original := kugouDesktopPlaylistPageProvider
	kugouDesktopPlaylistPageProvider = provider
	t.Cleanup(func() { kugouDesktopPlaylistPageProvider = original })
}

func TestCategoryPlaylistsUsesKugouDesktopPaginationAndSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withKugouDesktopPlaylistPageProvider(t, func(categoryID string, page, pageSize, sortID int) (core.KugouDesktopPlaylistPage, error) {
		if categoryID != "desktop:1085" || page != 2 || pageSize != 5 || sortID != 6 {
			t.Fatalf("unexpected provider arguments: %q page=%d pageSize=%d sort=%d", categoryID, page, pageSize, sortID)
		}
		return core.KugouDesktopPlaylistPage{
			Playlists: []model.Playlist{{ID: "3216573", Name: "官方频道歌单", Creator: "酷狗音乐", Source: "kugou", PlayCount: 3494000}},
			Total:     12,
			Page:      2,
			PageSize:  5,
		}, nil
	})

	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	RegisterMusicRoutes(router.Group(RoutePrefix), router.Group(RoutePrefix+"/config"))
	req := httptest.NewRequest("GET", RoutePrefix+"/category_playlists?source=kugou&category_id=desktop%3A1085&category_name=%E5%AE%98%E6%96%B9%E6%AD%8C%E5%8D%95&page=2&page_size=5&sort=6", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"官方频道歌单", "播放 349.4万 次", "第 2 / 3 页", "推荐", "最新", "最热", "好评", "热藏"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
	if !strings.Contains(body, `category-sort-tab is-active`) || !strings.Contains(body, `sort=6`) {
		t.Fatalf("response missing active Kugou sort state")
	}
	if strings.Contains(body, "未找到符合条件的资源") {
		t.Fatal("playlist result must not render a conflicting no-results message")
	}
}

func TestLoadKugouDesktopPlaylistPageClampsPastLastPage(t *testing.T) {
	var pages []int
	withKugouDesktopPlaylistPageProvider(t, func(categoryID string, page, pageSize, sortID int) (core.KugouDesktopPlaylistPage, error) {
		pages = append(pages, page)
		switch page {
		case 999:
			return core.KugouDesktopPlaylistPage{Page: page, PageSize: pageSize}, nil
		case 1:
			return core.KugouDesktopPlaylistPage{Total: 12, Page: 1, PageSize: pageSize}, nil
		case 3:
			return core.KugouDesktopPlaylistPage{
				Playlists: []model.Playlist{{ID: "last", Name: "末页歌单"}},
				Total:     12,
				Page:      3,
				PageSize:  pageSize,
			}, nil
		default:
			t.Fatalf("unexpected page %d", page)
			return core.KugouDesktopPlaylistPage{}, nil
		}
	})

	result, err := loadKugouDesktopPlaylistPage("desktop:1085", 999, 5, 5)
	if err != nil {
		t.Fatalf("loadKugouDesktopPlaylistPage() error = %v", err)
	}
	if result.Page != 3 || len(result.Playlists) != 1 || !reflect.DeepEqual(pages, []int{999, 1, 3}) {
		t.Fatalf("unexpected result/pages: result=%#v pages=%v", result, pages)
	}
}

func TestRenderIndexSupportsPrePaginatedPlaylistResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.SetHTMLTemplate(newTestTemplate(t))
	router.GET(RoutePrefix, func(c *gin.Context) {
		c.Set("IndexPaginationOverride", indexPaginationOverride{Page: 3, PageSize: 2, TotalCount: 5})
		renderIndex(c, nil, []model.Playlist{{ID: "last", Name: "最后一项", Source: "kugou"}}, "", []string{"kugou"}, "", "playlist", "", "", "", false, "", nil)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", RoutePrefix, nil))
	body := rec.Body.String()
	for _, want := range []string{"最后一项", "找到 <span class=\"count\">5</span>", "第 3 / 3 页", "显示 5 - 5 / 5"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q", want)
		}
	}
}
