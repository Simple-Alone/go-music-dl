package web

import (
	"os"
	"strings"
	"testing"
)

func TestKugouThemeProvidesVisiblePlayerControls(t *testing.T) {
	jsBytes, err := os.ReadFile("../../deploy/theme/kugou-dark.js")
	if err != nil {
		t.Fatalf("read kugou theme JS: %v", err)
	}
	cssBytes, err := os.ReadFile("../../deploy/theme/kugou-dark.css")
	if err != nil {
		t.Fatalf("read kugou theme CSS: %v", err)
	}

	js := string(jsBytes)
	for _, want := range []string{
		`id = "main-player-controls"`,
		`data-player-action="previous"`,
		`data-player-action="toggle"`,
		`data-player-action="next"`,
		`window.ap.toggle()`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("kugou theme JS missing %q", want)
		}
	}

	css := string(cssBytes)
	for _, want := range []string{
		".music-client-theme .main-player-controls",
		".music-client-theme .main-player-toggle",
		"left: 220px !important;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("kugou theme CSS missing %q", want)
		}
	}
}

func TestSongDetailCoversThemeShellAndSupportsEscape(t *testing.T) {
	jsBytes, err := os.ReadFile("../../deploy/theme/kugou-dark.js")
	if err != nil {
		t.Fatalf("read kugou theme JS: %v", err)
	}
	cssBytes, err := os.ReadFile("../../deploy/theme/kugou-dark.css")
	if err != nil {
		t.Fatalf("read kugou theme CSS: %v", err)
	}

	js := string(jsBytes)
	for _, want := range []string{
		`event.key !== "Escape"`,
		`window.VideoGen?.close()`,
		`closeButton.setAttribute("aria-label", "关闭歌曲详情")`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("kugou theme JS missing %q", want)
		}
	}

	css := string(cssBytes)
	for _, want := range []string{
		".music-client-theme .vg-overlay",
		"z-index: 10050;",
		".music-client-theme.vg-open .app-sidebar",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("kugou theme CSS missing %q", want)
		}
	}
}

func TestSongDetailHasAccessibleCloseControl(t *testing.T) {
	modalBytes, err := templateFS.ReadFile("templates/partials/videogen_modal.html")
	if err != nil {
		t.Fatalf("read videogen modal: %v", err)
	}
	jsBytes, err := templateFS.ReadFile("templates/static/js/videogen.js")
	if err != nil {
		t.Fatalf("read videogen JS: %v", err)
	}
	cssBytes, err := templateFS.ReadFile("templates/static/css/videogen.css")
	if err != nil {
		t.Fatalf("read videogen CSS: %v", err)
	}

	modal := string(modalBytes)
	for _, want := range []string{
		`role="dialog"`,
		`aria-modal="true"`,
		`<button type="button" class="vg-close"`,
		`aria-label="关闭歌曲详情"`,
	} {
		if !strings.Contains(modal, want) {
			t.Fatalf("videogen modal missing %q", want)
		}
	}

	if !strings.Contains(string(jsBytes), `event.key !== "Escape"`) {
		t.Fatal("videogen JS missing Escape close handling")
	}
	if !strings.Contains(string(cssBytes), "z-index: 10050;") {
		t.Fatal("videogen overlay should render above the themed application shell")
	}
}
