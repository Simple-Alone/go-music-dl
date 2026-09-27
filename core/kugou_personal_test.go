package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func withKugouPersonalTestState(t *testing.T, cookie string, server *httptest.Server) {
	t.Helper()
	originalCookie := CM.Get("kugou")
	originalEndpoint := kugouPersonalEndpoint
	originalClient := kugouPersonalHTTPClient
	originalNow := kugouPersonalNow
	CM.SetAll(map[string]string{"kugou": cookie})
	kugouPersonalEndpoint = server.URL
	kugouPersonalHTTPClient = server.Client()
	kugouPersonalNow = func() time.Time { return time.Unix(1_700_000_000, 123_000_000) }
	t.Cleanup(func() {
		CM.SetAll(map[string]string{"kugou": originalCookie})
		kugouPersonalEndpoint = originalEndpoint
		kugouPersonalHTTPClient = originalClient
		kugouPersonalNow = originalNow
	})
}

func TestGetKugouPersonalRecommendationsSignsAndMapsResponse(t *testing.T) {
	var requestBody kugouPersonalRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("x-router"); got != "persnfm.service.kugou.com" {
			t.Errorf("x-router = %q", got)
		}
		if got := r.URL.Query().Get("userid"); got != "42" {
			t.Errorf("userid = %q, want 42", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		query := make(map[string]string)
		for key, values := range r.URL.Query() {
			if key != "signature" && len(values) > 0 {
				query[key] = values[0]
			}
		}
		encodedBody, _ := json.Marshal(requestBody)
		if got, want := r.URL.Query().Get("signature"), signKugouPersonalRequest(query, encodedBody); got != want {
			t.Errorf("signature = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": 1,
			"error_code": 0,
			"data": {"cur_mark": 99999998, "song_list": [{
				"hash": "HASH128",
				"hash_128": "HASH128",
				"hash_320": "HASH320",
				"hash_flac": "HASHFLAC",
				"filesize_128": 5138176,
				"songname": "她说",
				"author_name": "林俊杰",
				"album_id": 1740660,
				"songid": "123",
				"mixsongid": 39611422,
				"time_length": 321,
				"bitrate": 128,
				"extname": "mp3",
				"privilege": 10,
				"trans_param": {"union_cover": "https://img.test/{size}/cover.jpg"},
				"relate_goods": [{"albumname": "她说", "album_id": "1740660"}]
			}]}
		}`))
	}))
	defer server.Close()
	withKugouPersonalTestState(t, "token=secret; userid=42; KUGOU_API_GUID=device-guid; dfid=test-dfid", server)

	result, err := GetKugouPersonalRecommendations(KugouPersonalRecommendOptions{
		Action:          "play",
		Hash:            "CURRENT_HASH",
		SongID:          "123",
		PlayTime:        87,
		Cursor:          "9988",
		RemainSongCount: 2,
		IsOverplay:      true,
	})
	if err != nil {
		t.Fatalf("GetKugouPersonalRecommendations() error = %v", err)
	}
	if requestBody.Action != "play" || requestBody.UserID != 42 || requestBody.Token != "secret" || requestBody.Hash != "CURRENT_HASH" || requestBody.SongID != "123" || requestBody.PlayTime != 87 || requestBody.Cursor != 9988 || requestBody.RemainSongCount != 2 || requestBody.IsOverplay != 1 {
		t.Fatalf("unexpected request body: %#v", requestBody)
	}
	if requestBody.Mid != "208752100769692998786554189566642396111" {
		t.Fatalf("fallback mid = %q", requestBody.Mid)
	}
	if result.Cursor != "99999998" {
		t.Fatalf("cursor = %q, want 99999998", result.Cursor)
	}
	if len(result.Songs) != 1 {
		t.Fatalf("songs = %d, want 1", len(result.Songs))
	}
	song := result.Songs[0]
	if song.ID != "HASH128" || song.Name != "她说" || song.Artist != "林俊杰" || song.Source != "kugou" {
		t.Fatalf("unexpected song: %#v", song)
	}
	if song.Album != "她说" || song.AlbumID != "1740660" || song.Cover != "https://img.test/240/cover.jpg" {
		t.Fatalf("unexpected album mapping: %#v", song)
	}
	if song.Extra["hq_hash"] != "HASH320" || song.Extra["sq_hash"] != "HASHFLAC" || song.Extra["privilege"] != "10" {
		t.Fatalf("unexpected song extra: %#v", song.Extra)
	}
}

func TestGetKugouPersonalRecommendationsRequiresLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("request must not be sent without login")
	}))
	defer server.Close()
	withKugouPersonalTestState(t, "userid=0", server)

	_, err := GetKugouPersonalRecommendations(KugouPersonalRecommendOptions{})
	if err != ErrKugouPersonalLoginRequired {
		t.Fatalf("error = %v, want %v", err, ErrKugouPersonalLoginRequired)
	}
}

func TestSignKugouPersonalRequestIgnoresExistingSignature(t *testing.T) {
	query := map[string]string{"b": "2", "a": "1"}
	signed := signKugouPersonalRequest(query, []byte(`{"x":1}`))
	query["signature"] = url.QueryEscape("ignored")
	if got := signKugouPersonalRequest(query, []byte(`{"x":1}`)); got != signed {
		t.Fatalf("signature changed after adding signature field: %q != %q", got, signed)
	}
	if len(strings.TrimSpace(signed)) != 32 {
		t.Fatalf("signature length = %d, want 32", len(signed))
	}
}
