package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultSilenceNoiseDB         = -45
	defaultMinimumSilenceDuration = 0.45
)

var (
	silenceStartPattern = regexp.MustCompile(`silence_start:\s*([0-9.]+)`)
	silenceEndPattern   = regexp.MustCompile(`silence_end:\s*([0-9.]+)`)
)

type ExtractAudioOptions struct {
	InputPath  string
	OutputPath string
	Overwrite  bool
	LogWriter  io.Writer
	Progress   ProgressFunc
	Duration   float64
}

type DetectSilenceOptions struct {
	AudioPath          string
	NoiseDB            float64
	MinSilenceDuration float64
	LogWriter          io.Writer
}

type SilenceRange struct {
	Start float64
	End   float64
}

func ExtractAudio(ctx context.Context, opts ExtractAudioOptions) error {
	return NewClient().ExtractAudio(ctx, opts)
}

func (*Client) ExtractAudio(ctx context.Context, opts ExtractAudioOptions) error {
	args, err := extractAudioArgs(opts)
	if err != nil {
		return err
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg executable not found in PATH")
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	if opts.LogWriter != nil || opts.Progress != nil {
		if err := runWithProgress(cmd, opts.LogWriter, opts.Progress, opts.Duration); err != nil {
			return fmt.Errorf("extract audio: %w", err)
		}

		return nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("extract audio: %w", err)
		}

		return fmt.Errorf("extract audio: %w: %s", err, message)
	}

	return nil
}

func DetectSilence(ctx context.Context, opts DetectSilenceOptions) ([]SilenceRange, error) {
	return NewClient().DetectSilence(ctx, opts)
}

func (*Client) DetectSilence(ctx context.Context, opts DetectSilenceOptions) ([]SilenceRange, error) {
	args, err := detectSilenceArgs(opts)
	if err != nil {
		return nil, err
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("ffmpeg executable not found in PATH")
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if opts.LogWriter != nil && len(output) > 0 {
		_, _ = opts.LogWriter.Write(output)
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, fmt.Errorf("detect silence: %w", err)
		}
		return nil, fmt.Errorf("detect silence: %w: %s", err, message)
	}

	return parseSilenceRanges(string(output)), nil
}

func detectSilenceArgs(opts DetectSilenceOptions) ([]string, error) {
	if opts.AudioPath == "" {
		return nil, errors.New("audio path is required")
	}

	inputInfo, err := os.Stat(opts.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("audio file: %w", err)
	}
	if inputInfo.IsDir() {
		return nil, errors.New("audio path must be a file")
	}

	noiseDB := opts.NoiseDB
	if noiseDB == 0 {
		noiseDB = defaultSilenceNoiseDB
	}
	minDuration := opts.MinSilenceDuration
	if minDuration <= 0 {
		minDuration = defaultMinimumSilenceDuration
	}

	return []string{
		"-hide_banner",
		"-nostdin",
		"-i", opts.AudioPath,
		"-af", fmt.Sprintf("silencedetect=noise=%.0fdB:d=%.3f", noiseDB, minDuration),
		"-f", "null",
		"-",
	}, nil
}

func parseSilenceRanges(output string) []SilenceRange {
	ranges := make([]SilenceRange, 0)
	start := -1.0
	for _, line := range strings.Split(output, "\n") {
		if match := silenceStartPattern.FindStringSubmatch(line); len(match) == 2 {
			if parsed, err := strconv.ParseFloat(match[1], 64); err == nil {
				start = parsed
			}
			continue
		}

		if match := silenceEndPattern.FindStringSubmatch(line); len(match) == 2 && start >= 0 {
			if end, err := strconv.ParseFloat(match[1], 64); err == nil && end > start {
				ranges = append(ranges, SilenceRange{Start: start, End: end})
			}
			start = -1
		}
	}

	return ranges
}

func extractAudioArgs(opts ExtractAudioOptions) ([]string, error) {
	if opts.InputPath == "" {
		return nil, errors.New("input path is required")
	}

	if opts.OutputPath == "" {
		return nil, errors.New("output path is required")
	}

	inputInfo, err := os.Stat(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("input file: %w", err)
	}

	if inputInfo.IsDir() {
		return nil, errors.New("input path must be a file")
	}

	if !opts.Overwrite {
		if _, err := os.Stat(opts.OutputPath); err == nil {
			return nil, errors.New("output file already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("output file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	args := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
	}

	if opts.Overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}

	args = append(
		args,
		"-i", opts.InputPath,
		"-vn",
		"-map", "a:0",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		opts.OutputPath,
	)

	return args, nil
}
