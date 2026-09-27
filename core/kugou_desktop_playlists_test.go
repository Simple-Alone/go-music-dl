package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func withKugouDesktopPlaylistTestServer(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	originalEndpoint := kugouDesktopPlaylistEndpoint
	originalClient := kugouDesktopPlaylistHTTPClient
	kugouDesktopPlaylistEndpoint = server.URL
	kugouDesktopPlaylistHTTPClient = server.Client()
	t.Cleanup(func() {
		server.Close()
		kugouDesktopPlaylistEndpoint = originalEndpoint
		kugouDesktopPlaylistHTTPClient = originalClient
	})
}

func TestGetKugouDesktopPlaylistCategoriesMapsOfficialGroups(t *testing.T) {
	withKugouDesktopPlaylistTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("c"); got != "0" {
			t.Errorf("category = %q, want 0", got)
		}
		_, _ = w.Write([]byte(`<!doctype html><html><body>
			<div class="album_hot_tag"><a title="经典">经典</a></div>
			<div class="album_class_list"><h3>默认</h3><dl>
				<dd><a href="/yueku/v8/special/index/index.html" title="全部">全部</a></dd>
			</dl></div>
			<div class="album_class_list"><h3>主题</h3><dl>
				<dd><a href="/yueku/v8/special/index/getData.js?cdn=cdn&t=5&c=1084" title="精选">精选</a></dd>
				<dd><a href="/yueku/v8/special/index/getData.js?cdn=cdn&t=5&c=12" title="经典">经典</a></dd>
			</dl></div>
		</body></html>`))
	}))

	categories, err := GetKugouDesktopPlaylistCategories()
	if err != nil {
		t.Fatalf("GetKugouDesktopPlaylistCategories() error = %v", err)
	}
	if len(categories) != 3 {
		t.Fatalf("categories = %d, want 3: %#v", len(categories), categories)
	}
	if categories[0].ID != "desktop:0" || categories[0].Name != "全部" || categories[0].Group != "默认" {
		t.Fatalf("unexpected all category: %#v", categories[0])
	}
	if categories[1].ID != "desktop:1084" || categories[1].Name != "精选" || categories[1].Group != "主题" {
		t.Fatalf("unexpected featured category: %#v", categories[1])
	}
	if !categories[2].Hot || categories[2].ID != "desktop:12" {
		t.Fatalf("unexpected hot category: %#v", categories[2])
	}
}

func TestGetKugouDesktopPlaylistPageMapsCardsAndPagination(t *testing.T) {
	withKugouDesktopPlaylistTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("c") != "1085" || query.Get("t") != "6" || query.Get("p") != "2" || query.Get("pagesize") != "5" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`<!doctype html><html><body>
			<ul id="special_list"><li>
				<a class="pic" title="测试频道歌单" href="/yueku/v8/special/single/3216573-5-1085.html">
					<img _src="http://imge.kugou.com/cover.jpg">
					<span class="sd sd_play"><i></i><span>349.4万</span></span>
				</a>
				<strong class="singer_a singer"><a>酷狗音乐官方歌单</a></strong>
			</li></ul>
			<script>document.write(new Page(2, 12, 5, 5).GetText());</script>
		</body></html>`))
	}))

	result, err := GetKugouDesktopPlaylistPage("desktop:1085", 2, 5, 6)
	if err != nil {
		t.Fatalf("GetKugouDesktopPlaylistPage() error = %v", err)
	}
	if result.Total != 12 || result.Page != 2 || result.PageSize != 5 {
		t.Fatalf("unexpected pagination: %#v", result)
	}
	if len(result.Playlists) != 1 {
		t.Fatalf("playlists = %d, want 1", len(result.Playlists))
	}
	playlist := result.Playlists[0]
	if playlist.ID != "3216573" || playlist.Name != "测试频道歌单" || playlist.Creator != "酷狗音乐官方歌单" {
		t.Fatalf("unexpected playlist: %#v", playlist)
	}
	if playlist.Cover != "https://imge.kugou.com/cover.jpg" || playlist.PlayCount != 3494000 {
		t.Fatalf("unexpected playlist media fields: %#v", playlist)
	}
	if playlist.Extra["category_id"] != "desktop:1085" || playlist.Extra["sort"] != "6" {
		t.Fatalf("unexpected playlist extra: %#v", playlist.Extra)
	}
}

func TestGetKugouDesktopPlaylistPageRejectsLegacyCategoryID(t *testing.T) {
	if _, err := GetKugouDesktopPlaylistPage("12:0", 1, 20, 5); err == nil {
		t.Fatal("expected legacy category ID to be rejected")
	}
}
