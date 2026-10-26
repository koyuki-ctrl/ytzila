package main

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

var ValidFormats = map[Format]bool{
	FormatMP4: true,
	FormatMP3: true,
}

var ValidQualities = map[Quality]bool{
	QualityLow:    true,
	QualityMedium: true,
	QualityHD:     true,
	QualityHigh:   true,
}
