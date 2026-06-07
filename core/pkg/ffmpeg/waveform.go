package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
)

const defaultWaveformSamplesPerSecond = 80

type WaveformOptions struct {
	InputPath        string
	SamplesPerSecond int
}

type Waveform struct {
	SamplesPerSecond int         `json:"samplesPerSecond"`
	Peaks            [][]float64 `json:"peaks"`
}

type audiowaveformOutput struct {
	Channels int       `json:"channels"`
	Data     []float64 `json:"data"`
}

func GenerateWaveform(ctx context.Context, opts WaveformOptions) (*Waveform, error) {
	return NewClient().GenerateWaveform(ctx, opts)
}

func (*Client) GenerateWaveform(ctx context.Context, opts WaveformOptions) (*Waveform, error) {
	if opts.InputPath == "" {
		return nil, errors.New("input path is required")
	}

	samplesPerSecond := opts.SamplesPerSecond
	if samplesPerSecond <= 0 {
		samplesPerSecond = defaultWaveformSamplesPerSecond
	}

	audiowaveformPath, err := exec.LookPath("audiowaveform")
	if err != nil {
		return nil, errors.New("audiowaveform executable not found in PATH")
	}

	file, err := os.CreateTemp("", "trimia-waveform-*.json")
	if err != nil {
		return nil, fmt.Errorf("create waveform temp file: %w", err)
	}
	outputPath := file.Name()
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close waveform temp file: %w", err)
	}
	defer os.Remove(outputPath)

	args := []string{
		"-i", opts.InputPath,
		"-o", outputPath,
		"--pixels-per-second", fmt.Sprintf("%d", samplesPerSecond),
		"--bits", "8",
	}
	cmd := exec.CommandContext(ctx, audiowaveformPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, fmt.Errorf("generate waveform: %w", err)
		}

		return nil, fmt.Errorf("generate waveform: %w: %s", err, message)
	}

	contents, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read waveform output: %w", err)
	}

	var raw audiowaveformOutput
	if err := json.Unmarshal(contents, &raw); err != nil {
		return nil, fmt.Errorf("parse waveform output: %w", err)
	}

	peaks, err := normalizeAudiowaveformData(raw)
	if err != nil {
		return nil, err
	}

	return &Waveform{
		SamplesPerSecond: samplesPerSecond,
		Peaks:            peaks,
	}, nil
}

func normalizeAudiowaveformData(raw audiowaveformOutput) ([][]float64, error) {
	if raw.Channels <= 0 {
		raw.Channels = 1
	}
	if len(raw.Data) == 0 {
		return nil, errors.New("waveform output is empty")
	}

	channelStride := raw.Channels * 2
	if len(raw.Data)%channelStride != 0 {
		return nil, errors.New("waveform output has invalid channel data")
	}

	maxValue := 0.0
	for _, value := range raw.Data {
		maxValue = max(maxValue, math.Abs(value))
	}
	if maxValue == 0 {
		maxValue = 1
	}

	peaks := make([][]float64, raw.Channels)
	for channel := 0; channel < raw.Channels; channel++ {
		peaks[channel] = make([]float64, 0, len(raw.Data)/raw.Channels)
	}

	for index := 0; index < len(raw.Data); index += channelStride {
		for channel := 0; channel < raw.Channels; channel++ {
			minValue := raw.Data[index+channel*2] / maxValue
			maxValue := raw.Data[index+channel*2+1] / maxValue
			peaks[channel] = append(peaks[channel], minValue, maxValue)
		}
	}

	return peaks, nil
}
