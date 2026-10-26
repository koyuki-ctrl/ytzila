package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"github.com/schollz/progressbar/v3"
)

type Deps struct {
	YtDlp   string
	FFmpeg  string
	FFprobe string
}

// withSpinner affiche un spinner pendant l'exécution de fn.
func withSpinner(label string, fn func() error) error {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(label),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionClearOnFinish(),
	)

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = bar.Add(1)
			}
		}
	}()

	err := fn()

	close(done)
	wg.Wait()
	_ = bar.Finish()
	_ = bar.Clear()

	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ %s\n", label)
	} else {
		fmt.Fprintf(os.Stderr, "✓ %s\n", label)
	}
	return err
}

func EnsureDependencies(ctx context.Context) (*Deps, error) {
	var yt, ff, fp *ytdlp.ResolvedInstall

	err := withSpinner("[1/3] yt-dlp", func() (err error) {
		yt, err = ytdlp.Install(ctx, nil)
		return
	})
	if err != nil {
		return nil, fmt.Errorf("yt-dlp installation failed: %w", err)
	}

	err = withSpinner("[2/3] ffmpeg", func() (err error) {
		ff, err = ytdlp.InstallFFmpeg(ctx, nil)
		return
	})
	if err != nil {
		return nil, fmt.Errorf("ffmpeg installation failed: %w", err)
	}

	err = withSpinner("[3/3] ffprobe", func() (err error) {
		fp, err = ytdlp.InstallFFprobe(ctx, nil)
		return
	})
	if err != nil {
		return nil, fmt.Errorf("ffprobe installation failed: %w", err)
	}

	return &Deps{
		YtDlp:   yt.Executable,
		FFmpeg:  ff.Executable,
		FFprobe: fp.Executable,
	}, nil
}

func Download(opts Options) error {
	ctx := context.Background()

	deps, err := EnsureDependencies(ctx)
	if err != nil {
		return err
	}

	dl := ytdlp.New().
		SetExecutable(deps.YtDlp).
		FFmpegLocation(deps.FFmpeg)

	dlBar := progressbar.DefaultBytes(-1, "Downloading")
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

	if _, err := dl.Run(ctx, opts.URL); err != nil {
		return err
	}
	_ = dlBar.Finish()
	return nil
}