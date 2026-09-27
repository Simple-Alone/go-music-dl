package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"golang.org/x/net/html"
)

const kugouDesktopPlaylistCategoryPrefix = "desktop:"

var (
	kugouDesktopPlaylistEndpoint   = "https://pc.service.kugou.com/yueku/v8/special/index/getData.js"
	kugouDesktopPlaylistHTTPClient = &http.Client{Timeout: 15 * time.Second}
	kugouDesktopPlaylistPageRE     = regexp.MustCompile(`new Page\(\s*\d+\s*,\s*(\d+)\s*,`)
	kugouDesktopPlaylistIDRE       = regexp.MustCompile(`/single/(\d+)`)
)

type KugouDesktopPlaylistPage struct {
	Playlists []model.Playlist
	Total     int
	Page      int
	PageSize  int
}

type KugouDesktopPlaylistSort struct {
	ID   int
	Name string
}

func KugouDesktopPlaylistSorts() []KugouDesktopPlaylistSort {
	return []KugouDesktopPlaylistSort{
		{ID: 5, Name: "推荐"},
		{ID: 7, Name: "最新"},
		{ID: 6, Name: "最热"},
		{ID: 2, Name: "好评"},
		{ID: 3, Name: "热藏"},
	}
}

func IsKugouDesktopPlaylistCategoryID(categoryID string) bool {
	return strings.HasPrefix(strings.TrimSpace(categoryID), kugouDesktopPlaylistCategoryPrefix)
}

func GetKugouDesktopPlaylistCategories() ([]model.PlaylistCategory, error) {
	_, document, err := fetchKugouDesktopPlaylistDocument("0", 1, 1, 5)
	if err != nil {
		return nil, err
	}

	hotNames := make(map[string]bool)
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "div" || !htmlHasClass(node, "album_hot_tag") {
			return
		}
		for _, link := range htmlDescendants(node, "a") {
			if name := htmlNodeText(link); name != "" {
				hotNames[name] = true
			}
		}
	})

	categories := make([]model.PlaylistCategory, 0, 100)
	seen := make(map[string]bool)
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "div" || !htmlHasClass(node, "album_class_list") {
			return
		}
		group := htmlNodeText(htmlFirstDescendant(node, "h3", ""))
		if group == "" {
			group = "其他"
		}
		for _, link := range htmlDescendants(node, "a") {
			name := strings.TrimSpace(htmlAttr(link, "title"))
			if name == "" {
				name = htmlNodeText(link)
			}
			channelID := kugouDesktopChannelID(htmlAttr(link, "href"))
			if name == "" || channelID == "" {
				continue
			}
			categoryID := kugouDesktopPlaylistCategoryPrefix + channelID
			if seen[categoryID] {
				continue
			}
			seen[categoryID] = true
			categories = append(categories, model.PlaylistCategory{
				ID:     categoryID,
				Name:   name,
				Group:  group,
				Source: "kugou",
				Hot:    hotNames[name],
				Extra:  map[string]string{"channel_id": channelID},
			})
		}
	})
	if len(categories) == 0 {
		return nil, errors.New("酷狗桌面频道没有返回分类")
	}
	return categories, nil
}

func GetKugouDesktopPlaylistPage(categoryID string, page, pageSize, sortID int) (KugouDesktopPlaylistPage, error) {
	channelID, err := parseKugouDesktopPlaylistCategoryID(categoryID)
	if err != nil {
		return KugouDesktopPlaylistPage{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if !validKugouDesktopPlaylistSort(sortID) {
		sortID = 5
	}

	body, document, err := fetchKugouDesktopPlaylistDocument(channelID, page, pageSize, sortID)
	if err != nil {
		return KugouDesktopPlaylistPage{}, err
	}
	playlists := parseKugouDesktopPlaylists(document, categoryID, sortID)
	total := len(playlists)
	if match := kugouDesktopPlaylistPageRE.FindSubmatch(body); len(match) == 2 {
		if parsed, parseErr := strconv.Atoi(string(match[1])); parseErr == nil && parsed >= total {
			total = parsed
		}
	}
	return KugouDesktopPlaylistPage{
		Playlists: playlists,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

func GetKugouDesktopCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	result, err := GetKugouDesktopPlaylistPage(categoryID, page, limit, 5)
	return result.Playlists, err
}

func fetchKugouDesktopPlaylistDocument(channelID string, page, pageSize, sortID int) ([]byte, *html.Node, error) {
	endpoint, err := url.Parse(kugouDesktopPlaylistEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("创建酷狗桌面频道请求失败：%w", err)
	}
	query := endpoint.Query()
	query.Set("c", channelID)
	query.Set("cdn", "cdn")
	query.Set("t", strconv.Itoa(sortID))
	query.Set("p", strconv.Itoa(page))
	query.Set("pagesize", strconv.Itoa(pageSize))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建酷狗桌面频道请求失败：%w", err)
	}
	req.Header.Set("User-Agent", UA_Common)
	req.Header.Set("Referer", "https://www.kugou.com/")
	resp, err := kugouDesktopPlaylistHTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("请求酷狗桌面频道失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("酷狗桌面频道返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("读取酷狗桌面频道失败：%w", err)
	}
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, fmt.Errorf("解析酷狗桌面频道失败：%w", err)
	}
	return body, document, nil
}

func parseKugouDesktopPlaylists(document *html.Node, categoryID string, sortID int) []model.Playlist {
	list := htmlFirstDescendantByID(document, "special_list")
	if list == nil {
		return nil
	}
	playlists := make([]model.Playlist, 0)
	for item := list.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "li" {
			continue
		}
		coverLink := htmlFirstDescendant(item, "a", "pic")
		if coverLink == nil {
			continue
		}
		name := strings.TrimSpace(htmlAttr(coverLink, "title"))
		match := kugouDesktopPlaylistIDRE.FindStringSubmatch(htmlAttr(coverLink, "href"))
		if name == "" || len(match) != 2 {
			continue
		}
		playlistID := match[1]
		cover := ""
		if image := htmlFirstDescendant(coverLink, "img", ""); image != nil {
			cover = strings.TrimSpace(htmlAttr(image, "_src"))
			if strings.HasPrefix(cover, "http://") {
				cover = "https://" + strings.TrimPrefix(cover, "http://")
			}
		}
		creator := ""
		for _, strong := range htmlDescendants(item, "strong") {
			if htmlHasClass(strong, "singer") {
				creator = htmlNodeText(strong)
				break
			}
		}
		playCount := 0
		if countNode := htmlFirstDescendant(item, "span", "sd_play"); countNode != nil {
			playCount = parseKugouDesktopCount(htmlNodeText(countNode))
		}
		playlists = append(playlists, model.Playlist{
			ID:        playlistID,
			Name:      name,
			Cover:     cover,
			PlayCount: playCount,
			Creator:   creator,
			Source:    "kugou",
			Link:      fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"category_id": categoryID,
				"sort":        strconv.Itoa(sortID),
			},
		})
	}
	return playlists
}

func parseKugouDesktopPlaylistCategoryID(categoryID string) (string, error) {
	categoryID = strings.TrimSpace(categoryID)
	if !IsKugouDesktopPlaylistCategoryID(categoryID) {
		return "", errors.New("无效的酷狗桌面频道分类")
	}
	channelID := strings.TrimSpace(strings.TrimPrefix(categoryID, kugouDesktopPlaylistCategoryPrefix))
	if channelID == "" {
		channelID = "0"
	}
	if _, err := strconv.Atoi(channelID); err != nil {
		return "", errors.New("无效的酷狗桌面频道分类")
	}
	return channelID, nil
}

func kugouDesktopChannelID(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if channelID := strings.TrimSpace(parsed.Query().Get("c")); channelID != "" {
		if _, err := strconv.Atoi(channelID); err == nil {
			return channelID
		}
	}
	if strings.Contains(parsed.Path, "/special/index/index.html") {
		return "0"
	}
	return ""
}

func validKugouDesktopPlaylistSort(sortID int) bool {
	for _, option := range KugouDesktopPlaylistSorts() {
		if option.ID == sortID {
			return true
		}
	}
	return false
}

func parseKugouDesktopCount(value string) int {
	value = strings.TrimSpace(value)
	multiplier := 1.0
	switch {
	case strings.HasSuffix(value, "亿"):
		multiplier = 100000000
		value = strings.TrimSuffix(value, "亿")
	case strings.HasSuffix(value, "万"):
		multiplier = 10000
		value = strings.TrimSuffix(value, "万")
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || number <= 0 {
		return 0
	}
	return int(number * multiplier)
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func htmlDescendants(node *html.Node, tag string) []*html.Node {
	result := make([]*html.Node, 0)
	walkHTML(node, func(candidate *html.Node) {
		if candidate.Type == html.ElementNode && candidate.Data == tag {
			result = append(result, candidate)
		}
	})
	return result
}

func htmlFirstDescendant(node *html.Node, tag, className string) *html.Node {
	var result *html.Node
	walkHTML(node, func(candidate *html.Node) {
		if result != nil || candidate.Type != html.ElementNode || candidate.Data != tag {
			return
		}
		if className == "" || htmlHasClass(candidate, className) {
			result = candidate
		}
	})
	return result
}

func htmlFirstDescendantByID(node *html.Node, id string) *html.Node {
	var result *html.Node
	walkHTML(node, func(candidate *html.Node) {
		if result == nil && candidate.Type == html.ElementNode && htmlAttr(candidate, "id") == id {
			result = candidate
		}
	})
	return result
}

func htmlHasClass(node *html.Node, className string) bool {
	for _, value := range strings.Fields(htmlAttr(node, "class")) {
		if value == className {
			return true
		}
	}
	return false
}

func htmlAttr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func htmlNodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var text strings.Builder
	walkHTML(node, func(candidate *html.Node) {
		if candidate.Type == html.TextNode {
			text.WriteString(candidate.Data)
			text.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(text.String()), " ")
}
