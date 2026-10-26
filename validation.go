package main

import 	(
	"regexp"
	"fmt"
)

var youtubeURLPattern = regexp.MustCompile(
	`(?i)^(?:https?://)?` +
		`(?:www\.|m\.|music\.)?` +
		`(?:` +
		`youtube\.com/(?:` +
		`watch\?(?:[^\s&#]*&)?v=[A-Za-z0-9_-]{11}` +
		`|embed/[A-Za-z0-9_-]{11}` +
		`|shorts/[A-Za-z0-9_-]{11}` +
		`|live/[A-Za-z0-9_-]{11}` +
		`|v/[A-Za-z0-9_-]{11}` +
		`)` +
		`|youtu\.be/[A-Za-z0-9_-]{11}` +
		`)` +
		`(?:[?&#][^\s]*)?$`,
)

func ValidateFormat(f Format) error {
	if !ValidFormats[f] {
		return fmt.Errorf("invalid format: %q (expected: mp3, mp4)", f)
	}
	return nil
}

func ValidateQuality(q Quality) error {
	if !ValidQualities[q] {
		return fmt.Errorf("invalid quality: %d (expected: 144, 360, 720, 1080)", q)
	}
	return nil
}

func IsYouTubeURL(s string) bool {
	return youtubeURLPattern.MatchString(s)
}