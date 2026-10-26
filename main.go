package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
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

			if !IsYouTubeURL(ctx.String("url")) {
				return fmt.Errorf("Youtube URL is only authorised")
			}

			opts := Options{
				URL:     ctx.String("url"),
				Format:  Format(ctx.String("format")),
				Quality: Quality(ctx.Int("quality")),
			}

			if err := ValidateFormat(opts.Format); err != nil {
				return err
			}
			if err := ValidateQuality(opts.Quality); err != nil {
				return err
			}

			return Download(opts)
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}