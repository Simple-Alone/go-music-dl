package core

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	kugouPersonalAppID     = "1005"
	kugouPersonalClientVer = "20489"
	kugouPersonalSignSalt  = "OIlwieks28dk2k092lksi2UIkp"
	kugouPersonalFakem     = "ca981cfc583a4c37f28d2d49000013c16a0a"
)

var (
	ErrKugouPersonalLoginRequired = errors.New("需要先登录酷狗账号")
	kugouPersonalEndpoint         = "https://gateway.kugou.com/v2/personal_recommend"
	kugouPersonalHTTPClient       = &http.Client{Timeout: 15 * time.Second}
	kugouPersonalNow              = time.Now
)

type KugouPersonalRecommendOptions struct {
	Action          string
	Hash            string
	SongID          string
	PlayTime        int
	Cursor          string
	RemainSongCount int
	IsOverplay      bool
}

type KugouPersonalRecommendResult struct {
	Songs  []model.Song
	Cursor string
}

type kugouPersonalRequest struct {
	AppID                 int    `json:"appid"`
	ClientTime            int64  `json:"clienttime"`
	Mid                   string `json:"mid"`
	Action                string `json:"action"`
	RecommendSourceLocked int    `json:"recommend_source_locked"`
	SongPoolID            int    `json:"song_pool_id"`
	CallerID              int    `json:"callerid"`
	MType                 int    `json:"m_type"`
	Platform              string `json:"platform"`
	AreaCode              int    `json:"area_code"`
	RemainSongCount       int    `json:"remain_songcnt"`
	ClientVer             int    `json:"clientver"`
	IsOverplay            int    `json:"is_overplay"`
	Mode                  string `json:"mode"`
	Fakem                 string `json:"fakem"`
	Key                   string `json:"key"`
	UserID                int64  `json:"userid"`
	KGUserID              int64  `json:"kguid"`
	Token                 string `json:"token"`
	Hash                  string `json:"hash,omitempty"`
	SongID                string `json:"songid,omitempty"`
	PlayTime              int    `json:"playtime,omitempty"`
	Cursor                int64  `json:"cur_mark,omitempty"`
}

type kugouPersonalResponse struct {
	Status    int    `json:"status"`
	ErrorCode int    `json:"error_code"`
	Error     string `json:"error"`
	Data      struct {
		Songs  []kugouPersonalSong `json:"song_list"`
		Cursor kugouPersonalString `json:"cur_mark"`
	} `json:"data"`
}

type kugouPersonalSong struct {
	Hash         string               `json:"hash"`
	Hash128      string               `json:"hash_128"`
	Hash320      string               `json:"hash_320"`
	HashFLAC     string               `json:"hash_flac"`
	FileSize     int64                `json:"filesize"`
	FileSize128  int64                `json:"filesize_128"`
	FileSize320  int64                `json:"filesize_320"`
	FileSizeFLAC int64                `json:"filesize_flac"`
	SongName     string               `json:"songname"`
	AuthorName   string               `json:"author_name"`
	FileName     string               `json:"filename"`
	AlbumName    string               `json:"album_name"`
	AlbumID      kugouPersonalString  `json:"album_id"`
	SongID       kugouPersonalString  `json:"songid"`
	MixSongID    kugouPersonalString  `json:"mixsongid"`
	Duration     int                  `json:"time_length"`
	Bitrate      int                  `json:"bitrate"`
	Ext          string               `json:"extname"`
	Privilege    int                  `json:"privilege"`
	TransParam   kugouPersonalTrans   `json:"trans_param"`
	RelateGoods  []kugouPersonalGoods `json:"relate_goods"`
}

type kugouPersonalTrans struct {
	Cover          string `json:"union_cover"`
	Ogg128Hash     string `json:"ogg_128_hash"`
	Ogg320Hash     string `json:"ogg_320_hash"`
	Ogg128FileSize int64  `json:"ogg_128_filesize"`
	Ogg320FileSize int64  `json:"ogg_320_filesize"`
}

type kugouPersonalGoods struct {
	AlbumName  string              `json:"albumname"`
	AlbumID    kugouPersonalString `json:"album_id"`
	Hash       string              `json:"hash"`
	Info       kugouPersonalTrans  `json:"info"`
	TransParam kugouPersonalTrans  `json:"trans_param"`
}

type kugouPersonalString string

func (s *kugouPersonalString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*s = kugouPersonalString(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*s = kugouPersonalString(number.String())
	return nil
}

func KugouPersonalRecommendationLoggedIn() bool {
	cookies := parseKugouCookie(CM.Get("kugou"))
	return kugouPersonalLoginValid(cookies)
}

func GetKugouPersonalRecommendations(options KugouPersonalRecommendOptions) (KugouPersonalRecommendResult, error) {
	cookie := CM.Get("kugou")
	cookies := parseKugouCookie(cookie)
	if !kugouPersonalLoginValid(cookies) {
		return KugouPersonalRecommendResult{}, ErrKugouPersonalLoginRequired
	}

	action := strings.TrimSpace(options.Action)
	if action != "play" {
		action = "login"
	}
	userID, _ := strconv.ParseInt(cookies["userid"], 10, 64)
	cursor, _ := strconv.ParseInt(strings.TrimSpace(options.Cursor), 10, 64)
	remainSongCount := options.RemainSongCount
	if remainSongCount < 0 {
		remainSongCount = 0
	}
	mid := strings.TrimSpace(cookies["KUGOU_API_MID"])
	if mid == "" {
		seed := firstKugouPersonalValue(cookies["KUGOU_API_GUID"], "go-music-dl:"+cookies["userid"])
		digest := md5.Sum([]byte(seed))
		mid = new(big.Int).SetBytes(digest[:]).String()
	}

	now := kugouPersonalNow()
	millis := now.UnixMilli()
	keyDigest := md5.Sum([]byte(kugouPersonalAppID + kugouPersonalSignSalt + kugouPersonalClientVer + strconv.FormatInt(millis, 10)))
	payload := kugouPersonalRequest{
		AppID:                 1005,
		ClientTime:            millis,
		Mid:                   mid,
		Action:                action,
		RecommendSourceLocked: 0,
		SongPoolID:            0,
		CallerID:              0,
		MType:                 1,
		Platform:              "ios",
		AreaCode:              1,
		RemainSongCount:       remainSongCount,
		ClientVer:             20489,
		IsOverplay:            boolToKugouPersonalInt(options.IsOverplay),
		Mode:                  "normal",
		Fakem:                 kugouPersonalFakem,
		Key:                   hex.EncodeToString(keyDigest[:]),
		UserID:                userID,
		KGUserID:              userID,
		Token:                 cookies["token"],
		Hash:                  strings.TrimSpace(options.Hash),
		SongID:                strings.TrimSpace(options.SongID),
		PlayTime:              options.PlayTime,
		Cursor:                cursor,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return KugouPersonalRecommendResult{}, fmt.Errorf("生成酷狗推荐请求失败：%w", err)
	}

	query := map[string]string{
		"dfid":       firstKugouPersonalValue(cookies["dfid"], "-"),
		"mid":        mid,
		"uuid":       "-",
		"appid":      kugouPersonalAppID,
		"clientver":  kugouPersonalClientVer,
		"clienttime": strconv.FormatInt(now.Unix(), 10),
		"token":      cookies["token"],
		"userid":     cookies["userid"],
	}
	query["signature"] = signKugouPersonalRequest(query, body)
	values := url.Values{}
	for key, value := range query {
		values.Set(key, value)
	}

	req, err := http.NewRequest(http.MethodPost, kugouPersonalEndpoint+"?"+values.Encode(), bytes.NewReader(body))
	if err != nil {
		return KugouPersonalRecommendResult{}, fmt.Errorf("创建酷狗推荐请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Android15-1070-11083-46-0-DiscoveryDRADProtocol-wifi")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("dfid", query["dfid"])
	req.Header.Set("clienttime", query["clienttime"])
	req.Header.Set("mid", mid)
	req.Header.Set("kg-rc", "1")
	req.Header.Set("kg-thash", "5d816a0")
	req.Header.Set("kg-rec", "1")
	req.Header.Set("kg-rf", "B9EDA08A64250DEFFBCADDEE00F8F25F")
	req.Header.Set("x-router", "persnfm.service.kugou.com")

	resp, err := kugouPersonalHTTPClient.Do(req)
	if err != nil {
		return KugouPersonalRecommendResult{}, fmt.Errorf("请求酷狗原生推荐失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return KugouPersonalRecommendResult{}, fmt.Errorf("酷狗原生推荐返回 HTTP %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return KugouPersonalRecommendResult{}, fmt.Errorf("读取酷狗原生推荐失败：%w", err)
	}
	var response kugouPersonalResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return KugouPersonalRecommendResult{}, fmt.Errorf("解析酷狗原生推荐失败：%w", err)
	}
	if response.ErrorCode != 0 || strings.TrimSpace(response.Error) != "" {
		return KugouPersonalRecommendResult{}, fmt.Errorf("酷狗原生推荐失败：error_code=%d error=%s", response.ErrorCode, response.Error)
	}

	songs := mapKugouPersonalSongs(response.Data.Songs)
	if len(songs) == 0 {
		return KugouPersonalRecommendResult{}, errors.New("酷狗原生推荐暂时没有返回歌曲")
	}
	return KugouPersonalRecommendResult{
		Songs:  songs,
		Cursor: strings.TrimSpace(string(response.Data.Cursor)),
	}, nil
}

func boolToKugouPersonalInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseKugouCookie(cookie string) map[string]string {
	values := make(map[string]string)
	for _, part := range strings.Split(cookie, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func kugouPersonalLoginValid(cookies map[string]string) bool {
	userID, err := strconv.ParseInt(strings.TrimSpace(cookies["userid"]), 10, 64)
	return err == nil && userID > 0 && strings.TrimSpace(cookies["token"]) != ""
}

func signKugouPersonalRequest(query map[string]string, body []byte) string {
	keys := make([]string, 0, len(query))
	for key := range query {
		if key != "signature" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var signed strings.Builder
	signed.WriteString(kugouPersonalSignSalt)
	for _, key := range keys {
		signed.WriteString(key)
		signed.WriteByte('=')
		signed.WriteString(query[key])
	}
	signed.Write(body)
	signed.WriteString(kugouPersonalSignSalt)
	digest := md5.Sum([]byte(signed.String()))
	return hex.EncodeToString(digest[:])
}

func mapKugouPersonalSongs(items []kugouPersonalSong) []model.Song {
	songs := make([]model.Song, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		hash := firstKugouPersonalValue(item.Hash, item.Hash128, item.Hash320, item.HashFLAC)
		name := strings.TrimSpace(item.SongName)
		if hash == "" || name == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}

		album := strings.TrimSpace(item.AlbumName)
		albumID := strings.TrimSpace(string(item.AlbumID))
		cover := strings.TrimSpace(item.TransParam.Cover)
		if len(item.RelateGoods) > 0 {
			goods := item.RelateGoods[0]
			album = firstKugouPersonalValue(album, goods.AlbumName)
			albumID = firstKugouPersonalValue(albumID, string(goods.AlbumID))
			cover = firstKugouPersonalValue(cover, goods.TransParam.Cover, goods.Info.Cover)
		}
		cover = strings.Replace(cover, "{size}", "240", 1)
		size := item.FileSize
		if size <= 0 {
			size = item.FileSize128
		}
		bitrate := item.Bitrate
		if bitrate <= 0 && item.Duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(item.Duration))
		}
		ext := strings.TrimSpace(item.Ext)
		if ext == "" {
			ext = "mp3"
		}
		songs = append(songs, model.Song{
			ID:       hash,
			Name:     name,
			Artist:   strings.TrimSpace(item.AuthorName),
			Album:    album,
			AlbumID:  albumID,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Source:   "kugou",
			Ext:      ext,
			Cover:    cover,
			Link:     "https://www.kugou.com/song/#hash=" + hash,
			Extra: map[string]string{
				"hash":           hash,
				"file_hash":      firstKugouPersonalValue(item.Hash128, item.Hash),
				"hq_hash":        item.Hash320,
				"sq_hash":        item.HashFLAC,
				"ogg_128_hash":   item.TransParam.Ogg128Hash,
				"ogg_320_hash":   item.TransParam.Ogg320Hash,
				"audio_id":       string(item.SongID),
				"album_audio_id": string(item.MixSongID),
				"album_id":       albumID,
				"privilege":      strconv.Itoa(item.Privilege),
			},
		})
	}
	return songs
}

func firstKugouPersonalValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
