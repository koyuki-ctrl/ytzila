package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"github.com/schollz/progressbar/v3"
	"github.com/urfave/cli/v2"
)

type Format string
type Quality int

type Options struct {
	URL     string
	Format  Format
	Quality Quality
}

const (
	FormatMP4 Format = "mp4"
	FormatMP3 Format = "mp3"
)

const (
	QualityLow    Quality = 144
	QualityMedium Quality = 360
	QualityHD     Quality = 720
	QualityHigh   Quality = 1080
)

var validFormats = map[Format]bool{
	FormatMP4: true,
	FormatMP3: true,
}

var validQualities = map[Quality]bool{
	QualityLow:    true,
	QualityMedium: true,
	QualityHD:     true,
	QualityHigh:   true,
}

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

func validateFormat(f Format) error {
	if !validFormats[f] {
		return fmt.Errorf("invalid format: %q (expected: mp3, mp4)", f)
	}
	return nil
}

func validateQuality(q Quality) error {
	if !validQualities[q] {
		return fmt.Errorf("invalid quality: %d (expected: 144, 360, 720, 1080)", q)
	}
	return nil
}

func isYouTubeURL(s string) bool {
	return youtubeURLPattern.MatchString(s)
}

func installWithSpinner(ctx context.Context) error {
	spinner := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Installation des dépendances..."),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWriter(os.Stderr),
	)

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("installation échouée : %v", r)
			}
		}()
		ytdlp.MustInstallAll(ctx)
		done <- nil
	}()

	for {
		select {
		case err := <-done:
			_ = spinner.Finish()
			if err != nil {
				return err
			}
			return nil
		default:
			_ = spinner.Add(1)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func main() {
	if err := installWithSpinner(context.TODO()); err != nil {
		log.Fatalf("Échec de l'installation : %v", err)
	}

	cli.HelpFlag = &cli.BoolFlag{
		Name:    "help",
		Aliases: []string{"h"},
		Usage:   "Show usage of ytzila",
	}

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "Show version of ytzila",
	}

	app := &cli.App{
		Name:    "ytzila",
		Usage:   "Youtube downloader based on yt-dlp",
		Version: "v0.0.1",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "url",
				Aliases: []string{"u"},
				Usage:   "Load youtube `URL`",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   string(FormatMP4),
				Usage:   "Set format of your video/audio `FORMAT`",
			},
			&cli.IntFlag{
				Name:    "quality",
				Aliases: []string{"q"},
				Value:   int(QualityHigh),
				Usage:   "Set quality of your video/audio `QUALITY`",
			},
		},
		Action: func(ctx *cli.Context) error {
			if !ctx.IsSet("url") || ctx.String("url") == "" {
				_ = cli.ShowAppHelp(ctx)
				return fmt.Errorf("flag --url is required")
			}

			if !isYouTubeURL(ctx.String("url")) {
				return fmt.Errorf("Youtube URL is only authorised")
			}

			opts := Options{
				URL:     ctx.String("url"),
				Format:  Format(ctx.String("format")),
				Quality: Quality(ctx.Int("quality")),
			}

			if err := validateFormat(opts.Format); err != nil {
				return err
			}
			if err := validateQuality(opts.Quality); err != nil {
				return err
			}

			dl := ytdlp.New()

			dlBar := progressbar.DefaultBytes(-1, "Téléchargement")
			dl.ProgressFunc(100*time.Millisecond, func(p ytdlp.ProgressUpdate) {
				if p.TotalBytes > 0 {
					_ = dlBar.Set(p.DownloadedBytes)
				} else {
					_ = dlBar.Add(p.DownloadedBytes)
				}
			})

			switch opts.Format {
			case FormatMP3:
				dl.ExtractAudio().
					AudioFormat("mp3").
					AudioQuality("0").
					EmbedMetadata().
					EmbedThumbnail().
					RestrictFilenames().
					Output("%(title)s.%(ext)s")

			case FormatMP4:
				dl.MergeOutputFormat("mp4").
					Format(fmt.Sprintf(
						"bestvideo[height<=%d][ext=mp4]+bestaudio[ext=m4a]/best[height<=%d]",
						int(opts.Quality), int(opts.Quality),
					)).
					EmbedThumbnail().
					RestrictFilenames().
					Output("%(title)s.%(ext)s")
			}

			_, err := dl.Run(context.TODO(), opts.URL)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				_ = dlBar.Finish()
			}
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}