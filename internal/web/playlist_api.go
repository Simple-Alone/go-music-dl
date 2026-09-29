package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

const (
	playlistAPIMaxPlaylists = 120
	playlistAPIMaxSongs     = 1000
)

type playlistAPIProviders struct {
	sourceNames       func() []string
	sourceDescription func(string) string
	search            func(string) core.SearchPlaylistFunc
	categories        func(string) core.PlaylistCategoriesFunc
	category          func(string) core.CategoryPlaylistsFunc
	recommend         func(string) func() ([]model.Playlist, error)
	userPlaylists     func(string) core.UserPlaylistsFunc
	detail            func(string) func(string) ([]model.Song, error)
}

type playlistAPIPlatform struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Search        bool   `json:"search"`
	Categories    bool   `json:"categories"`
	Recommend     bool   `json:"recommend"`
	UserPlaylists bool   `json:"user_playlists"`
}

type playlistAPICategory struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
	Hot   bool   `json:"hot,omitempty"`
}

type playlistAPIPlaylist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Cover       string `json:"cover,omitempty"`
	Source      string `json:"source"`
	TrackCount  int    `json:"track_count,omitempty"`
}

type playlistAPISong struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Cover    string `json:"cover,omitempty"`
	Source   string `json:"source"`
	Duration int    `json:"duration,omitempty"`
	IsVIP    bool   `json:"is_vip,omitempty"`
	Extra    any    `json:"extra,omitempty"`
}

func defaultPlaylistAPIProviders() playlistAPIProviders {
	return playlistAPIProviders{
		sourceNames:       core.GetPlaylistSourceNames,
		sourceDescription: core.GetSourceDescription,
		search:            core.GetPlaylistSearchFunc,
		categories:        core.GetPlaylistCategoriesFunc,
		category:          core.GetCategoryPlaylistsFunc,
		recommend:         core.GetRecommendFunc,
		userPlaylists:     core.GetUserPlaylistsFunc,
		detail:            core.GetPlaylistDetailFunc,
	}
}

func RegisterPlaylistAPIRoutes(api *gin.RouterGroup) {
	registerPlaylistAPIRoutes(api, defaultPlaylistAPIProviders())
}

func registerPlaylistAPIRoutes(api *gin.RouterGroup, providers playlistAPIProviders) {
	playlistAPI := api.Group("/api/playlist")

	playlistAPI.GET("/sources", func(c *gin.Context) {
		sources := make([]playlistAPIPlatform, 0)
		if providers.sourceNames == nil {
			c.JSON(http.StatusOK, gin.H{"sources": sources})
			return
		}
		for _, source := range providers.sourceNames() {
			source = strings.ToLower(strings.TrimSpace(source))
			if source == "" {
				continue
			}
			name := source
			if providers.sourceDescription != nil {
				name = strings.TrimSpace(providers.sourceDescription(source))
				if name == "" {
					name = source
				}
			}
			sources = append(sources, playlistAPIPlatform{
				ID:            source,
				Name:          name,
				Search:        providers.search != nil && providers.search(source) != nil,
				Categories:    providers.categories != nil && providers.categories(source) != nil && providers.category != nil && providers.category(source) != nil,
				Recommend:     providers.recommend != nil && providers.recommend(source) != nil,
				UserPlaylists: providers.userPlaylists != nil && providers.userPlaylists(source) != nil,
			})
		}
		c.JSON(http.StatusOK, gin.H{"sources": sources})
	})

	playlistAPI.GET("/categories", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		if providers.categories == nil || providers.categories(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		categories, err := providers.categories(source)()
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"categories": normalizePlaylistAPICategories(categories)})
	})

	playlistAPI.GET("/search", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		keyword := strings.TrimSpace(c.Query("q"))
		if keyword == "" || len(keyword) > 120 {
			playlistAPIError(c, http.StatusBadRequest, "invalid_query")
			return
		}
		if providers.search == nil || providers.search(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		playlists, err := providers.search(source)(keyword)
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"playlists": normalizePlaylistAPIPlaylists(playlists, source, playlistAPIMaxPlaylists)})
	})

	playlistAPI.GET("/category", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		categoryID := strings.TrimSpace(c.Query("category_id"))
		if categoryID == "" || len(categoryID) > 120 {
			playlistAPIError(c, http.StatusBadRequest, "invalid_category")
			return
		}
		page, pageOK := playlistAPIPositiveInt(c.Query("page"), 1, 10000)
		pageSize, pageSizeOK := playlistAPIPositiveInt(c.Query("page_size"), 60, 100)
		if !pageOK || !pageSizeOK {
			playlistAPIError(c, http.StatusBadRequest, "invalid_pagination")
			return
		}
		if providers.category == nil || providers.category(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		playlists, err := providers.category(source)(categoryID, page, pageSize)
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"playlists": normalizePlaylistAPIPlaylists(playlists, source, pageSize)})
	})

	playlistAPI.GET("/recommend", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		if providers.recommend == nil || providers.recommend(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		playlists, err := providers.recommend(source)()
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"playlists": normalizePlaylistAPIPlaylists(playlists, source, 60)})
	})

	playlistAPI.GET("/user", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		page, pageOK := playlistAPIPositiveInt(c.Query("page"), 1, 10000)
		limit, limitOK := playlistAPIPositiveInt(c.Query("limit"), 100, 100)
		if !pageOK || !limitOK {
			playlistAPIError(c, http.StatusBadRequest, "invalid_pagination")
			return
		}
		if providers.userPlaylists == nil || providers.userPlaylists(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		playlists, err := providers.userPlaylists(source)(page, limit)
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"playlists": normalizePlaylistAPIPlaylists(playlists, source, limit)})
	})

	playlistAPI.GET("/songs", func(c *gin.Context) {
		source, ok := playlistAPISource(c, providers)
		if !ok {
			return
		}
		playlistID := strings.TrimSpace(c.Query("id"))
		if playlistID == "" || len(playlistID) > 240 {
			playlistAPIError(c, http.StatusBadRequest, "invalid_playlist")
			return
		}
		if providers.detail == nil || providers.detail(source) == nil {
			playlistAPIError(c, http.StatusNotFound, "unsupported_capability")
			return
		}
		songs, err := providers.detail(source)(playlistID)
		if err != nil {
			playlistAPIError(c, http.StatusBadGateway, "upstream_failed")
			return
		}
		c.JSON(http.StatusOK, gin.H{"songs": normalizePlaylistAPISongs(songs, source, playlistAPIMaxSongs)})
	})
}

func playlistAPISource(c *gin.Context, providers playlistAPIProviders) (string, bool) {
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if source == "" || len(source) > 40 || providers.sourceNames == nil {
		playlistAPIError(c, http.StatusBadRequest, "invalid_source")
		return "", false
	}
	for _, candidate := range providers.sourceNames() {
		if strings.EqualFold(strings.TrimSpace(candidate), source) {
			return source, true
		}
	}
	playlistAPIError(c, http.StatusBadRequest, "invalid_source")
	return "", false
}

func playlistAPIPositiveInt(raw string, fallback, maximum int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, false
	}
	if value > maximum {
		value = maximum
	}
	return value, true
}

func playlistAPIError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"error": code})
}

func normalizePlaylistAPICategories(categories []model.PlaylistCategory) []playlistAPICategory {
	result := make([]playlistAPICategory, 0, len(categories))
	for _, category := range categories {
		name := strings.TrimSpace(category.Name)
		id := strings.TrimSpace(category.ID)
		if id == "" {
			id = name
		}
		if id == "" || name == "" {
			continue
		}
		result = append(result, playlistAPICategory{ID: id, Name: name, Group: strings.TrimSpace(category.Group), Hot: category.Hot})
	}
	return result
}

func normalizePlaylistAPIPlaylists(playlists []model.Playlist, fallbackSource string, limit int) []playlistAPIPlaylist {
	if limit < 1 || limit > playlistAPIMaxPlaylists {
		limit = playlistAPIMaxPlaylists
	}
	result := make([]playlistAPIPlaylist, 0, min(len(playlists), limit))
	for _, playlist := range playlists {
		if len(result) >= limit {
			break
		}
		id := strings.TrimSpace(playlist.ID)
		if id == "" {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(playlist.Source))
		if source == "" {
			source = fallbackSource
		}
		result = append(result, playlistAPIPlaylist{
			ID:          id,
			Name:        strings.TrimSpace(playlist.Name),
			Description: strings.TrimSpace(playlist.Description),
			Cover:       strings.TrimSpace(playlist.Cover),
			Source:      source,
			TrackCount:  playlist.TrackCount,
		})
	}
	return result
}

func normalizePlaylistAPISongs(songs []model.Song, fallbackSource string, limit int) []playlistAPISong {
	if limit < 1 || limit > playlistAPIMaxSongs {
		limit = playlistAPIMaxSongs
	}
	result := make([]playlistAPISong, 0, min(len(songs), limit))
	for _, song := range songs {
		if len(result) >= limit {
			break
		}
		id := strings.TrimSpace(song.ID)
		if id == "" {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(song.Source))
		if source == "" {
			source = fallbackSource
		}
		result = append(result, playlistAPISong{
			ID:       id,
			Name:     strings.TrimSpace(song.Name),
			Artist:   strings.TrimSpace(song.Artist),
			Album:    strings.TrimSpace(song.Album),
			Cover:    strings.TrimSpace(song.Cover),
			Source:   source,
			Duration: song.Duration,
			IsVIP:    song.IsVIP,
			Extra:    song.Extra,
		})
	}
	return result
}
