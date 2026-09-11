package web

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

const (
	maxGuessArtists = 6
	maxGuessSongs   = 60
)

type guessYouLikeData struct {
	Enabled bool
	Seeds   []string
	Message string
}

var guessSearchFuncProvider = func(source string) core.SearchFunc {
	return core.GetSearchFunc(source)
}

func normalizeGuessArtists(values []string) []string {
	artists := make([]string, 0, maxGuessArtists)
	seen := make(map[string]struct{}, maxGuessArtists)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		artists = append(artists, value)
		if len(artists) == maxGuessArtists {
			break
		}
	}
	return artists
}

func guessExcludedIDs(values []string) map[string]struct{} {
	excluded := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			excluded[value] = struct{}{}
		}
	}
	return excluded
}

func guessShuffleSeed(artists []string, refresh string) int64 {
	if strings.TrimSpace(refresh) == "" {
		refresh = time.Now().UTC().Format("2006-01-02")
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(artists, "\x00")))
	_, _ = h.Write([]byte("\x00" + refresh))
	return int64(h.Sum64())
}

func loadGuessSongs(artists []string, excluded map[string]struct{}, refresh string) ([]model.Song, error) {
	type searchResult struct {
		songs []model.Song
		err   error
	}

	results := make(chan searchResult, len(artists))
	var wg sync.WaitGroup
	for _, artist := range artists {
		artist := artist
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn := guessSearchFuncProvider("kugou")
			if fn == nil {
				results <- searchResult{err: fmt.Errorf("酷狗音乐源不可用")}
				return
			}
			songs, err := fn(artist)
			results <- searchResult{songs: songs, err: err}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	candidates := make([]model.Song, 0, maxGuessSongs)
	seen := make(map[string]struct{})
	failed := 0
	for result := range results {
		if result.err != nil {
			failed++
			continue
		}
		for _, song := range result.songs {
			id := strings.TrimSpace(song.ID)
			if id == "" || strings.TrimSpace(song.Name) == "" {
				continue
			}
			if _, skip := excluded[id]; skip {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			song.Source = "kugou"
			candidates = append(candidates, song)
		}
	}
	if len(candidates) == 0 && failed == len(artists) {
		return nil, fmt.Errorf("酷狗推荐暂时不可用，请稍后重试")
	}

	rng := rand.New(rand.NewSource(guessShuffleSeed(artists, refresh)))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	if len(candidates) > maxGuessSongs {
		candidates = candidates[:maxGuessSongs]
	}
	return candidates, nil
}

func registerGuessYouLikeRoute(api *gin.RouterGroup) {
	api.GET("/guess_you_like", func(c *gin.Context) {
		artists := normalizeGuessArtists(c.QueryArray("artists"))
		page := guessYouLikeData{Enabled: true, Seeds: artists}
		c.Set("GuessYouLike", page)

		if len(artists) == 0 {
			page.Message = "播放几首喜欢的歌曲后，这里会出现为你挑选的音乐。"
			c.Set("GuessYouLike", page)
			renderIndex(c, nil, nil, "", []string{"kugou"}, "", "song", "", "", "", false, "", nil)
			return
		}

		songs, err := loadGuessSongs(artists, guessExcludedIDs(c.QueryArray("exclude")), c.Query("refresh"))
		if err != nil {
			page.Message = err.Error()
		} else if len(songs) == 0 {
			page.Message = "这一批没有找到合适的歌曲，试试换一批。"
		}
		c.Set("GuessYouLike", page)
		renderIndex(c, songs, nil, "", []string{"kugou"}, "", "song", "", "", "", false, "", nil)
	})
}
