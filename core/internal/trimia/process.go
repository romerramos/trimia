package trimia

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"romerramos/trimia/internal/transcription"
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

	Transcriber         transcription.Transcriber
	TranscriberProvider transcription.Provider

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
	ID       string
	Start    float64
	End      float64
	Text     string
	Included bool
	Words    []transcription.Word
}

type AnalyzeOptions struct {
	InputPath string
	AudioPath string

	Transcriber         transcription.Transcriber
	TranscriberProvider transcription.Provider

	RemoveSilence     bool
	RemoveFillerWords bool

	Language       string
	DetectLanguage bool

	PreRoll  *float64
	PostRoll *float64
	MergeGap *float64

	KeepTempFiles bool
	LogDir        string
	Progress      ProgressFunc
}

type AnalysisResult struct {
	InputPath           string
	AudioPath           string
	TranscriberProvider transcription.Provider

	OriginalTranscript string
	CleanTranscript    string

	FillerWords []transcription.Word
	Segments    []Segment

	InputDurationSeconds           float64
	EstimatedOutputDurationSeconds float64
	EstimatedRemovedSeconds        float64
	EstimatedRemovedPercent        float64
}

type RenderOptions struct {
	InputPath  string
	OutputPath string
	Segments   []Segment

	PreRoll  *float64
	PostRoll *float64
	MergeGap *float64

	Overwrite bool
	LogDir    string
	Progress  ProgressFunc

	RenderMode  string
	VideoPreset string
	VideoCRF    int
	AudioRate   string
}

type RenderResult struct {
	InputPath  string
	OutputPath string

	InputDurationSeconds  float64
	OutputDurationSeconds float64
	RemovedSeconds        float64
	RemovedPercent        float64
}

type ProcessResult struct {
	InputPath           string
	OutputPath          string
	AudioPath           string
	TranscriberProvider transcription.Provider

	OriginalTranscript string
	CleanTranscript    string

	FillerWords []transcription.Word
	Segments    []Segment

	InputDurationSeconds  float64
	OutputDurationSeconds float64
	RemovedSeconds        float64
	RemovedPercent        float64
}

func Process(ctx context.Context, opts ProcessOptions) (*ProcessResult, error) {
	return newPipeline(opts.Transcriber).Run(ctx, opts)
}

func Analyze(ctx context.Context, opts AnalyzeOptions) (*AnalysisResult, error) {
	return newPipeline(opts.Transcriber).Analyze(ctx, opts)
}

func Render(ctx context.Context, opts RenderOptions) (*RenderResult, error) {
	return newPipeline(nil).Render(ctx, opts)
}

func validateProcessOptions(opts ProcessOptions) error {
	if opts.InputPath == "" {
		return errors.New("input path is required")
	}

	if opts.OutputPath == "" {
		return errors.New("output path is required")
	}

	if opts.Transcriber == nil {
		return errors.New("transcriber is required")
	}

	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	return nil
}

func validateAnalyzeOptions(opts AnalyzeOptions) error {
	if opts.InputPath == "" {
		return errors.New("input path is required")
	}

	if opts.Transcriber == nil {
		return errors.New("transcriber is required")
	}

	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	return nil
}

func validateRenderOptions(opts RenderOptions) error {
	if opts.InputPath == "" {
		return errors.New("input path is required")
	}

	if opts.OutputPath == "" {
		return errors.New("output path is required")
	}

	if len(includedSegments(opts.Segments)) == 0 {
		return errors.New("at least one included segment is required")
	}

	if _, err := os.Stat(opts.InputPath); err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	return nil
}

func processToAnalyzeOptions(opts ProcessOptions) AnalyzeOptions {
	return AnalyzeOptions{
		InputPath:           opts.InputPath,
		Transcriber:         opts.Transcriber,
		TranscriberProvider: opts.TranscriberProvider,
		RemoveSilence:       opts.RemoveSilence,
		RemoveFillerWords:   opts.RemoveFillerWords,
		Language:            opts.Language,
		DetectLanguage:      opts.DetectLanguage,
		PreRoll:             opts.PreRoll,
		PostRoll:            opts.PostRoll,
		MergeGap:            opts.MergeGap,
		KeepTempFiles:       opts.KeepTempFiles,
		LogDir:              opts.LogDir,
		Progress:            opts.Progress,
	}
}

func processToRenderOptions(opts ProcessOptions, segments []Segment) RenderOptions {
	return RenderOptions{
		InputPath:   opts.InputPath,
		OutputPath:  opts.OutputPath,
		Segments:    segments,
		PreRoll:     opts.PreRoll,
		PostRoll:    opts.PostRoll,
		MergeGap:    opts.MergeGap,
		Overwrite:   opts.Overwrite,
		LogDir:      opts.LogDir,
		Progress:    opts.Progress,
		RenderMode:  opts.RenderMode,
		VideoPreset: opts.VideoPreset,
		VideoCRF:    opts.VideoCRF,
		AudioRate:   opts.AudioRate,
	}
}

func applyDefaults(opts ProcessOptions) ProcessOptions {
	opts.TranscriberProvider = transcription.ProviderOrDefault(opts.TranscriberProvider)

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
	file, err := os.CreateTemp("", "trimia-audio-*.wav")
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

func toSegments(cleanSegments []transcription.Segment) []Segment {
	segments := make([]Segment, 0, len(cleanSegments))
	for i, segment := range cleanSegments {
		segments = append(segments, Segment{
			ID:       fmt.Sprintf("seg_%03d", i+1),
			Start:    segment.Start,
			End:      segment.End,
			Text:     segment.Text,
			Included: true,
			Words:    segment.Words,
		})
	}

	return normalizeSegments(segments)

}

func normalizeSegments(segments []Segment) []Segment {
	if len(segments) < 2 {
		return segments
	}

	sort.SliceStable(segments, func(i, j int) bool {
		return segments[i].Start < segments[j].Start
	})

	normalized := make([]Segment, 0, len(segments))
	for _, segment := range segments {
		if segment.End <= segment.Start {
			continue
		}

		if len(normalized) > 0 {
			previous := &normalized[len(normalized)-1]
			if segment.Start < previous.End {
				previous.End = segment.Start
			}
			if previous.End <= previous.Start {
				normalized = normalized[:len(normalized)-1]
			}
		}

		normalized = append(normalized, segment)
	}

	return normalized
}

func toFFmpegSegments(segments []Segment) []ffmpeg.Segment {
	included := includedSegments(segments)
	ffmpegSegments := make([]ffmpeg.Segment, 0, len(included))
	for _, segment := range included {
		ffmpegSegments = append(ffmpegSegments, ffmpeg.Segment{
			Start: segment.Start,
			End:   segment.End,
		})
	}

	return ffmpegSegments
}

func includedSegments(segments []Segment) []Segment {
	included := make([]Segment, 0, len(segments))
	for _, segment := range segments {
		if segment.Included {
			included = append(included, segment)
		}
	}

	return included
}

func estimateOutputDuration(segments []Segment, preRoll, postRoll, mergeGap float64) float64 {
	ffmpegSegments := toFFmpegSegments(segments)
	if len(ffmpegSegments) == 0 {
		return 0
	}

	prepared := make([]ffmpeg.Segment, 0, len(ffmpegSegments))
	for _, segment := range ffmpegSegments {
		start := segment.Start - preRoll
		if start < 0 {
			start = 0
		}
		prepared = append(prepared, ffmpeg.Segment{Start: start, End: segment.End + postRoll})
	}
	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].Start < prepared[j].Start
	})

	// This mirrors ffmpeg segment merging closely enough for UI estimates.
	merged := make([]ffmpeg.Segment, 0, len(prepared))
	for _, segment := range prepared {
		if len(merged) == 0 {
			merged = append(merged, segment)
			continue
		}

		last := &merged[len(merged)-1]
		if segment.Start <= last.End+mergeGap {
			if segment.End > last.End {
				last.End = segment.End
			}
			continue
		}

		merged = append(merged, segment)
	}

	duration := 0.0
	for _, segment := range merged {
		duration += segment.End - segment.Start
	}

	return duration
}

func DefaultOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	return filepath.Join(filepath.Dir(inputPath), base+"_trimia.mp4")
}
