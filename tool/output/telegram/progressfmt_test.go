package telegram

import (
	"strings"
	"testing"
)

func TestRenderTaskProgressMatchesAria2Frame(t *testing.T) {
	got := renderTaskProgress(taskProgressView{
		Filename:   "Sample.2028.mkv",
		Percent:    50,
		Downloaded: 512 * 1024 * 1024,
		Total:      1 * 1024 * 1024 * 1024,
		SpeedBps:   2.18 * 1024 * 1024,
		ETASeconds: 166, // 2m46s
		GID:        "tg12345",
	})
	want := "" +
		"*Filename:* `Sample.2028.mkv`\n" +
		"`[●●●●●●○○○○○○○] 50.00 %`\n" +
		"*Downloaded:* 512.00 MB *of* 1.00 GB\n" +
		"*Speed:* 2.18 MB/s\n" +
		"*ETA:* 2 m 46 s\n" +
		"*GID:* `tg12345`"
	if got != want {
		t.Errorf("renderTaskProgress mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderTaskProgressUnknownSizeAndSpeed(t *testing.T) {
	got := renderTaskProgress(taskProgressView{
		Filename:   "Clip 2028",
		Percent:    10,
		Downloaded: 5 * 1024 * 1024,
		Total:      0,
		SpeedBps:   0,
		GID:        "",
	})
	if strings.Contains(got, "*of*") {
		t.Errorf("unknown total must not render '*of*': %q", got)
	}
	if strings.Contains(got, "*Speed:*") || strings.Contains(got, "*ETA:*") {
		t.Errorf("zero speed must not render Speed/ETA lines: %q", got)
	}
	if !strings.Contains(got, "*GID:* `-`") {
		t.Errorf("empty GID must render '-': %q", got)
	}
}

func TestRenderTaskProgressSpeedWithoutETA(t *testing.T) {
	got := renderTaskProgress(taskProgressView{
		Filename:   "Movie.2028.mkv",
		Percent:    25,
		Downloaded: 100,
		Total:      400,
		SpeedBps:   1024,
		ETASeconds: 0,
		GID:        "g1",
	})
	if !strings.Contains(got, "*ETA:* ") {
		t.Errorf("speed without ETA must still render the ETA line: %q", got)
	}
	if !strings.Contains(got, "Unable to estimate") {
		t.Errorf("speed without ETA should fall back to 'Unable to estimate': %q", got)
	}
}

func TestRenderTaskProgressStripsBackticks(t *testing.T) {
	got := renderTaskProgress(taskProgressView{
		Filename: "Weird`name`.mkv",
		GID:      "g`1",
	})
	if strings.Contains(got, "Weird`name`") || strings.Contains(got, "g`1") {
		t.Errorf("backticks must be stripped from code spans: %q", got)
	}
}

func TestAriaETA(t *testing.T) {
	cases := map[int64]string{
		0:     "Unable to estimate",
		166:   "2 m 46 s",
		14872: "4 h 7 m 52 s",
		90061: "1 d 1 h 1 m 1 s",
	}
	for in, want := range cases {
		if got := ariaETA(in); got != want {
			t.Errorf("ariaETA(%d) = %q, want %q", in, got, want)
		}
	}
}
