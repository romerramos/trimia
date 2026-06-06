package trimia

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/pkg/ffmpeg"
)

const (
	defaultPreRoll  = 0.03
	defaultPostRoll = 0.06
	defaultMergeGap = 0.12
)

type ProcessOptions struct {
	InputPath  string
	OutputPath string

	DeepgramAPIKey string

	RemoveSilence     bool
	RemoveFillerWords bool

	Language       string
	DetectLanguage bool

	PreRoll  *float64
	PostRoll *float64
	MergeGap *float64

	Overwrite     bool
	KeepTempFiles bool
	LogDir        string
	Progress      ProgressFunc
	RenderMode    string
	VideoPreset   string
	VideoCRF      int
	AudioRate     string
}

type ProgressFunc func(phase string, percent float64)

type Segment struct {
	Start float64
	End   float64
	Text  string
}

type ProcessResult struct {
	InputPath  string
	OutputPath string
	AudioPath  string

	OriginalTranscript string
	CleanTranscript    string

	FillerWords []deepgram.Word
	Segments    []Segment

	InputDurationSeconds  float64
	OutputDurationSeconds float64
	RemovedSeconds        float64
	RemovedPercent        float64
}

func Process(ctx context.Context, opts ProcessOptions) (*ProcessResult, error) {
	return newPipeline(opts.DeepgramAPIKey).Run(ctx, opts)
}

func validateProcessOptions(opts ProcessOptions) error {
	if opts.InputPath == "" {
		return errors.New("input path is required")
	}

	if opts.OutputPath == "" {
		return errors.New("output path is required")
	}

	if opts.DeepgramAPIKey == "" {
		return errors.New("deepgram api key is required")
	}

	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	return nil
}

func applyDefaults(opts ProcessOptions) ProcessOptions {
	if !opts.RemoveSilence && !opts.RemoveFillerWords {
		opts.RemoveSilence = true
		opts.RemoveFillerWords = true
	}

	if !opts.DetectLanguage && opts.Language == "" {
		opts.DetectLanguage = true
	}

	return opts
}

func valueOrDefault(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}

	return *value
}

func createTempAudioPath() (string, error) {
	file, err := os.CreateTemp("", "trimia-audio-*.mp3")
	if err != nil {
		return "", fmt.Errorf("create temp audio file: %w", err)
	}

	path := file.Name()
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temp audio file: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("remove temp audio placeholder: %w", err)
	}

	return path, nil
}

func toSegments(cleanSegments []deepgram.CleanSegment) []Segment {
	segments := make([]Segment, 0, len(cleanSegments))
	for _, segment := range cleanSegments {
		segments = append(segments, Segment{
			Start: segment.Start,
			End:   segment.End,
			Text:  segment.Text,
		})
	}

	return segments
}

func toFFmpegSegments(segments []Segment) []ffmpeg.Segment {
	ffmpegSegments := make([]ffmpeg.Segment, 0, len(segments))
	for _, segment := range segments {
		ffmpegSegments = append(ffmpegSegments, ffmpeg.Segment{
			Start: segment.Start,
			End:   segment.End,
		})
	}

	return ffmpegSegments
}

func DefaultOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return filepath.Join(filepath.Dir(inputPath), base+"_trimia.mp4")
}
