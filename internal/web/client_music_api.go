package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type clientMusicAPIProviders struct {
	kugouLoggedIn func() bool
	kugouPersonal func(core.KugouPersonalRecommendOptions) (core.KugouPersonalRecommendResult, error)
	validate      func(*model.Song) bool
	switchSource  func(string, string, string, string, int) (*model.Song, float64, error)
}

func defaultClientMusicAPIProviders() clientMusicAPIProviders {
	return clientMusicAPIProviders{
		kugouLoggedIn: core.KugouPersonalRecommendationLoggedIn,
		kugouPersonal: core.GetKugouPersonalRecommendations,
		validate:      core.ValidatePlayable,
		switchSource:  findBestSwitchSong,
	}
}

func RegisterClientMusicAPIRoutes(api *gin.RouterGroup) {
	registerClientMusicAPIRoutes(api, defaultClientMusicAPIProviders())
}

func registerClientMusicAPIRoutes(api *gin.RouterGroup, providers clientMusicAPIProviders) {
	api.GET("/api/recommend/kugou/songs", func(c *gin.Context) {
		if providers.kugouLoggedIn == nil || !providers.kugouLoggedIn() {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "login_required", "mode": "unavailable", "songs": []playlistAPISong{}})
			return
		}

		action := strings.ToLower(strings.TrimSpace(c.Query("action")))
		if action == "" {
			action = "login"
		}
		if action != "login" && action != "play" {
			clientMusicAPIError(c, "invalid_action")
			return
		}
		if len(c.Query("hash")) > 240 || len(c.Query("song_id")) > 240 || len(c.Query("cursor")) > 240 || len(c.QueryArray("exclude")) > 100 {
			clientMusicAPIError(c, "invalid_playback_state")
			return
		}
		playTime, playTimeOK := clientMusicAPINonNegativeInt(c.Query("play_time"), 24*60*60)
		remainSongCount, remainOK := clientMusicAPINonNegativeInt(c.Query("remain_songcnt"), playlistAPIMaxSongs)
		if !playTimeOK || !remainOK {
			clientMusicAPIError(c, "invalid_playback_state")
			return
		}

		if providers.kugouPersonal == nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "upstream_failed", "mode": "native", "songs": []playlistAPISong{}})
			return
		}
		result, err := providers.kugouPersonal(core.KugouPersonalRecommendOptions{
			Action:          action,
			Hash:            strings.TrimSpace(c.Query("hash")),
			SongID:          strings.TrimSpace(c.Query("song_id")),
			PlayTime:        playTime,
			Cursor:          strings.TrimSpace(c.Query("cursor")),
			RemainSongCount: remainSongCount,
			IsOverplay:      c.Query("overplay") == "1",
		})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "upstream_failed", "mode": "native", "songs": []playlistAPISong{}})
			return
		}
		songs := filterGuessSongs(result.Songs, guessExcludedIDs(c.QueryArray("exclude")))
		c.JSON(http.StatusOK, gin.H{
			"ok":     true,
			"mode":   "native",
			"cursor": strings.TrimSpace(result.Cursor),
			"songs":  normalizePlaylistAPISongs(songs, "kugou", maxGuessSongs),
		})
	})

	api.GET("/api/playback/resolve", func(c *gin.Context) {
		song := lyricSongFromQuery(c)
		song.Name = strings.TrimSpace(song.Name)
		song.Artist = strings.TrimSpace(song.Artist)
		song.Source = strings.ToLower(strings.TrimSpace(song.Source))
		song.IsVIP = c.Query("is_vip") == "1"
		if song.ID == "" || song.Name == "" || song.Source == "" || len(song.ID) > 240 || len(song.Name) > 240 || len(song.Artist) > 240 || len(song.Album) > 240 || len(song.Source) > 40 || len(c.Query("extra")) > 8192 || core.GetDownloadFunc(song.Source) == nil {
			clientMusicAPIError(c, "invalid_track")
			return
		}

		forceFallback := c.Query("force_fallback") == "1"
		restricted := song.IsVIP || core.IsKugouRestrictedSong(song)
		if !forceFallback && providers.validate != nil && providers.validate(song) {
			clientMusicAPIResolved(c, song, song.Source, false, "", 1)
			return
		}

		if providers.switchSource != nil {
			selected, score, err := providers.switchSource(song.Name, song.Artist, song.Source, "", song.Duration)
			if err == nil && selected != nil {
				reason := "unavailable"
				if restricted {
					reason = "restricted"
				}
				clientMusicAPIResolved(c, selected, song.Source, true, reason, score)
				return
			}
		}

		reason := "unavailable"
		if restricted {
			reason = "restricted"
		}
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "no_playable_source", "fallback_reason": reason})
	})
}

func clientMusicAPINonNegativeInt(raw string, maximum int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > maximum {
		return 0, false
	}
	return value, true
}

func clientMusicAPIError(c *gin.Context, code string) {
	c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": code})
}

func clientMusicAPIResolved(c *gin.Context, song *model.Song, originalSource string, fallback bool, reason string, score float64) {
	tracks := normalizePlaylistAPISongs([]model.Song{*song}, song.Source, 1)
	if len(tracks) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "no_playable_source", "fallback_reason": reason})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"original_source": originalSource,
		"playback_source": tracks[0].Source,
		"fallback":        fallback,
		"fallback_reason": reason,
		"score":           score,
		"track":           tracks[0],
	})
}
