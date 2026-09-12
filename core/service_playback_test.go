package core

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestIsKugouRestrictedSong(t *testing.T) {
	tests := []struct {
		name      string
		song      *model.Song
		restricted bool
	}{
		{name: "vip privilege", song: &model.Song{Source: "kugou", Extra: map[string]string{"privilege": "10"}}, restricted: true},
		{name: "paid quality privilege", song: &model.Song{Source: "kugou", Extra: map[string]string{"privilege": "8"}}, restricted: true},
		{name: "free privilege", song: &model.Song{Source: "kugou", Extra: map[string]string{"privilege": "0"}}},
		{name: "other source", song: &model.Song{Source: "qq", Extra: map[string]string{"privilege": "10"}}},
		{name: "missing metadata", song: &model.Song{Source: "kugou"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKugouRestrictedSong(tt.song); got != tt.restricted {
				t.Fatalf("IsKugouRestrictedSong() = %v, want %v", got, tt.restricted)
			}
		})
	}
}

func TestIsLikelyIncompleteAudio(t *testing.T) {
	tests := []struct {
		name       string
		size       int64
		duration   int
		incomplete bool
	}{
		{name: "full 128 kbps track", size: 3_840_000, duration: 240},
		{name: "sixty second preview", size: 960_000, duration: 240, incomplete: true},
		{name: "unknown size", size: 0, duration: 240},
		{name: "short track", size: 320_000, duration: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLikelyIncompleteAudio(tt.size, tt.duration); got != tt.incomplete {
				t.Fatalf("IsLikelyIncompleteAudio(%d, %d) = %v, want %v", tt.size, tt.duration, got, tt.incomplete)
			}
		})
	}
}

func TestCalcSongSimilarityRejectsDifferentArtist(t *testing.T) {
	if got := CalcSongSimilarity("晴天", "周杰伦", "晴天", "张信哲"); got != 0 {
		t.Fatalf("different artist similarity = %f, want 0", got)
	}

	if got := CalcSongSimilarity("晴天", "周杰伦", "晴天", "周杰伦/温岚"); got < 0.8 {
		t.Fatalf("matching primary artist similarity = %f, want at least 0.8", got)
	}
}
