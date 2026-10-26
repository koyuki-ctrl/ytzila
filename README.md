# ytzila

A CLI tool to download YouTube videos using yt-dlp.

## Install

```bash
go build
```
## Usage

```bash
./ytzila --url "https://www.youtube.com/watch?v=VIDEO_ID"
./ytzila --url "https://www.youtube.com/watch?v=VIDEO_ID" --format mp3
./ytzila --url "https://www.youtube.com/watch?v=VIDEO_ID" --quality 720
```

## Supported formats
- mp4
- mp3

## Supported quality
- 144
- 360
- 720
- 1080