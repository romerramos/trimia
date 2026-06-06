package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func ProbeDuration(ctx context.Context, path string) (float64, error) {
	return NewClient().ProbeDuration(ctx, path)
}

func (*Client) ProbeDuration(ctx context.Context, path string) (float64, error) {
	if path == "" {
		return 0, errors.New("path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("media file: %w", err)
	}

	if info.IsDir() {
		return 0, errors.New("path must be a file")
	}

	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, errors.New("ffprobe executable not found in PATH")
	}

	cmd := exec.CommandContext(
		ctx,
		ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return 0, fmt.Errorf("probe duration: %w", err)
		}

		return 0, fmt.Errorf("probe duration: %w: %s", err, message)
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}

	return duration, nil
}
