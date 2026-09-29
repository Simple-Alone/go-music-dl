# Roadmap

## Current Stage

Improve Web discovery so repeated listening is not limited to a small fixed result set.

## In Progress

- None.

## Completed

- Web, TUI, desktop, and mobile application entry points.
- Multi-source song, playlist, and album search.
- Playlist categories, user playlists, local music, and local collections.
- Kugou native personalized song recommendations for logged-in Web users.
- Playback-history recommendations for logged-out users and explicit fallback after native request failures.
- Kugou-only recommendation candidates while preserving the existing cross-source playback fallback.
- Continuous Kugou native FM playback with cursor feedback, automatic prefetch when two tracks remain, and incremental page/player queue updates.
- Kugou desktop playlist-channel taxonomy with 83 current categories across default, theme, language, style, era, mood, and scene groups.
- Kugou channel browsing with recommendation, newest, hottest, highest-rated, and most-favorited sorts, plus server-side pagination and boundary clamping.
- Kugou desktop channel cards mapped to the existing playlist-detail flow with creator, cover, and compact play-count metadata.
- Read-only local-client APIs for platform capabilities, categories, search, recommendations, user playlists, playlist details, and playback metadata.
- Local-client Kugou personalized-song API with cursor playback feedback, plus playable-source resolution that reuses cross-platform similarity, duration, and media probes.

## Next

- Add pagination and deduplication to recommended playlists.

## Blockers

- None confirmed.

## Recent Verification

- 2026-09-28: Focused client recommendation, playback resolution, and playlist API tests passed; `core` and `cmd/music-dl` passed in the required suite, and `internal/web` passed on an isolated rerun after one pre-existing temporary-index cleanup race.
- 2026-09-28: A logged-in local service returned 5 native Kugou recommendations and 5 continuation songs; a real `privilege=10` track resolved to a same-title, same-artist, same-duration Kuwo track with similarity `1.0`.
- 2026-09-28: `go test ./core ./internal/web ./cmd/music-dl` passed after adding the local-client playlist APIs.
- 2026-09-28: A local preview exposed 12 platform capability records, returned all 83 Kugou categories, and completed a real category-to-playlist-to-songs request chain.
- 2026-09-27: `go test ./core ./internal/web ./cmd/music-dl` passed.
- 2026-09-27: Focused Kugou desktop playlist-category, channel pagination, rendering, and invalid-page clamping tests passed.
- 2026-09-27: Real Kugou desktop data exposed 83 categories; the sampled official-playlist channel returned 395 playlists across 20 pages, and page 999 clamped to page 20 with 15 cards.
- 2026-09-27: A sampled migrated channel playlist opened through the existing detail route and returned 136 songs.
- 2026-09-27: Focused Kugou personal recommendation tests passed in `core`.
- 2026-09-27: Focused Guess You Like route and client tests passed in `internal/web`.
- 2026-09-27: A real logged-in Kugou request returned 5 initial songs; a cursor/playback-feedback request returned 5 additional, non-duplicate songs.
- 2026-09-27: The pre-migration audit found 19 categories from the legacy mobile `api/v3/tag/list` feed and a hard-coded page-1, 120-item category result limit.
