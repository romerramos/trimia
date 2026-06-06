package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultAudioBitrate = "192k"

type ExtractAudioOptions struct {
	InputPath  string
	OutputPath string
	Bitrate    string
	Overwrite  bool
	LogWriter  io.Writer
	Progress   ProgressFunc
	Duration   float64
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

	bitrate := opts.Bitrate
	if bitrate == "" {
		bitrate = defaultAudioBitrate
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
		"-c:a", "libmp3lame",
		"-b:a", bitrate,
		opts.OutputPath,
	)

	return args, nil
}
