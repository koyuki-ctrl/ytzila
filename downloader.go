package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// ---------------------------------------------------------------------------
// Résolution des binaires : PATH -> cache ytzila -> téléchargement
// ---------------------------------------------------------------------------

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// cacheDir retourne ~/.cache/ytzila/bin (ou l'équivalent selon l'OS).
func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ytzila", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// findInPath cherche le binaire dans le PATH du système.
func findInPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	return p, err == nil
}

// findInCache cherche le binaire dans le cache de ytzila.
func findInCache(name string) (string, bool) {
	dir, err := cacheDir()
	if err != nil {
		return "", false
	}
	p := filepath.Join(dir, exeName(name))
	if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
		return p, true
	}
	return "", false
}

// ytDlpAssetName retourne le nom du fichier release de yt-dlp pour l'OS courant.
func ytDlpAssetName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "yt-dlp_linux_aarch64", nil
		}
		return "yt-dlp_linux", nil
	case "darwin":
		return "yt-dlp_macos", nil
	case "windows":
		return "yt-dlp.exe", nil
	}
	return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
}

// downloadFile télécharge url vers dest, avec reprise (.part) et sans timeout global.
func downloadFile(ctx context.Context, url, dest, label string) error {
	part := dest + ".part"

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
		// Pas de Timeout global : le corps peut être lent, seul ctx décide.
	}

	var offset int64
	if st, err := os.Stat(part); err == nil {
		offset = st.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusOK: // le serveur ignore Range : on repart de zéro
		offset = 0
		flags |= os.O_TRUNC
	case http.StatusPartialContent: // reprise
		flags |= os.O_APPEND
	case http.StatusRequestedRangeNotSatisfiable: // .part déjà complet
		return finalize(part, dest)
	default:
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}

	total := resp.ContentLength
	if total > 0 {
		total += offset
	}
	bar := progressbar.NewOptions64(total,
		progressbar.OptionSetDescription(label),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionClearOnFinish(),
	)
	_ = bar.Set64(offset)

	_, copyErr := io.Copy(io.MultiWriter(f, bar), resp.Body)
	closeErr := f.Close()
	_ = bar.Finish()

	if copyErr != nil {
		return copyErr // le .part est conservé pour la reprise
	}
	if closeErr != nil {
		return closeErr
	}
	return finalize(part, dest)
}

func finalize(part, dest string) error {
	if err := os.Chmod(part, 0o755); err != nil {
		return err
	}
	return os.Rename(part, dest)
}

// downloadWithRetry réessaie en reprenant le téléchargement à chaque échec.
func downloadWithRetry(ctx context.Context, url, dest, label string, attempts int) error {
	var err error
	for i := 1; i <= attempts; i++ {
		if err = downloadFile(ctx, url, dest, label); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintf(os.Stderr, "\n⚠ %s: tentative %d/%d échouée (%v), reprise...\n", label, i, attempts, err)
		time.Sleep(2 * time.Second)
	}
	return err
}

// resolveYtDlp : PATH -> cache -> téléchargement dans le cache.
func resolveYtDlp(ctx context.Context) (string, error) {
	if p, ok := findInPath("yt-dlp"); ok {
		fmt.Fprintf(os.Stderr, "✓ [1/3] yt-dlp (système: %s)\n", p)
		return p, nil
	}
	if p, ok := findInCache("yt-dlp"); ok {
		fmt.Fprintf(os.Stderr, "✓ [1/3] yt-dlp (cache: %s)\n", p)
		return p, nil
	}

	asset, err := ytDlpAssetName()
	if err != nil {
		return "", err
	}
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, exeName("yt-dlp"))
	url := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + asset

	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := downloadWithRetry(dlCtx, url, dest, "[1/3] yt-dlp", 5); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "✓ [1/3] yt-dlp (téléchargé: %s)\n", dest)
	return dest, nil
}

// resolveViaGoYtdlp : PATH -> cache ytzila -> go-ytdlp (ffmpeg/ffprobe).
func resolveViaGoYtdlp(
	ctx context.Context,
	step, name string,
	install func(context.Context) (*ytdlp.ResolvedInstall, error),
) (string, error) {
	if p, ok := findInPath(name); ok {
		fmt.Fprintf(os.Stderr, "✓ %s %s (système: %s)\n", step, name, p)
		return p, nil
	}
	if p, ok := findInCache(name); ok {
		fmt.Fprintf(os.Stderr, "✓ %s %s (cache: %s)\n", step, name, p)
		return p, nil
	}

	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	var res *ytdlp.ResolvedInstall
	err := withSpinner(step+" "+name, func() (err error) {
		res, err = install(dlCtx)
		return
	})
	if err != nil {
		return "", err
	}
	return res.Executable, nil
}

func EnsureDependencies(ctx context.Context) (*Deps, error) {
	yt, err := resolveYtDlp(ctx)
	if err != nil {
		return nil, fmt.Errorf("yt-dlp unavailable: %w", err)
	}

	ff, err := resolveViaGoYtdlp(ctx, "[2/3]", "ffmpeg",
		func(c context.Context) (*ytdlp.ResolvedInstall, error) {
			return ytdlp.InstallFFmpeg(c, nil)
		})
	if err != nil {
		return nil, fmt.Errorf("ffmpeg unavailable: %w", err)
	}

	fp, err := resolveViaGoYtdlp(ctx, "[3/3]", "ffprobe",
		func(c context.Context) (*ytdlp.ResolvedInstall, error) {
			return ytdlp.InstallFFprobe(c, nil)
		})
	if err != nil {
		return nil, fmt.Errorf("ffprobe unavailable: %w", err)
	}

	return &Deps{YtDlp: yt, FFmpeg: ff, FFprobe: fp}, nil
}

// ---------------------------------------------------------------------------
// Dossier de destination selon l'OS
// ---------------------------------------------------------------------------

// linuxDownloadsDir lit XDG_DOWNLOAD_DIR (variable d'environnement, puis
// ~/.config/user-dirs.dirs). Gère les systèmes localisés ("Téléchargements").
func linuxDownloadsDir(home string) string {
	if p := os.Getenv("XDG_DOWNLOAD_DIR"); p != "" {
		return p
	}

	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(cfg, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR=") {
			continue
		}
		v := strings.Trim(strings.TrimPrefix(line, "XDG_DOWNLOAD_DIR="), `"`)
		v = strings.ReplaceAll(v, "$HOME", home)
		return v
	}
	return ""
}

// downloadsDir retourne le dossier de téléchargements de l'utilisateur :
//   - Linux   : XDG_DOWNLOAD_DIR (ex. ~/Downloads ou ~/Téléchargements)
//   - macOS   : ~/Downloads
//   - Windows : %USERPROFILE%\Downloads
//
// Si le dossier est introuvable, retourne le dossier courant.
func downloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	dir := filepath.Join(home, "Downloads")
	if runtime.GOOS == "linux" {
		if p := linuxDownloadsDir(home); p != "" {
			dir = p
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "."
	}
	return dir
}

// ---------------------------------------------------------------------------
// Téléchargement de la vidéo / audio
// ---------------------------------------------------------------------------

func Download(opts Options) error {
	ctx := context.Background()

	deps, err := EnsureDependencies(ctx)
	if err != nil {
		return err
	}

	outDir := downloadsDir()
	outTmpl := filepath.Join(outDir, "%(title)s.%(ext)s")

	// Exécute le binaire résolu (PATH, cache ou fraîchement téléchargé).
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
			Output(outTmpl)

	case FormatMP4:
		dl.MergeOutputFormat("mp4").
			Format(fmt.Sprintf(
				"bestvideo[height<=%d][ext=mp4]+bestaudio[ext=m4a]/best[height<=%d]",
				int(opts.Quality), int(opts.Quality),
			)).
			EmbedThumbnail().
			RestrictFilenames().
			Output(outTmpl)
	}

	if _, err := dl.Run(ctx, opts.URL); err != nil {
		return err
	}
	_ = dlBar.Finish()
	fmt.Fprintf(os.Stderr, "✓ Fichier enregistré dans : %s\n", outDir)
	return nil
}
