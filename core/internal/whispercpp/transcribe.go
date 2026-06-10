package whispercpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultBinaryPath = "whisper-cli"

type Options struct {
	BinaryPath string
	ModelPath  string
}

type Client struct {
	BinaryPath string
	ModelPath  string
}

type TranscribeOptions struct {
	AudioPath      string
	Language       string
	DetectLanguage bool
}

type TranscriptionResponse struct {
	SystemInfo    string              `json:"systeminfo"`
	Model         Model               `json:"model"`
	Params        Params              `json:"params"`
	Result        Result              `json:"result"`
	Transcription []TranscriptionItem `json:"transcription"`
}

type Model struct {
	Type         string `json:"type"`
	Multilingual bool   `json:"multilingual"`
}

type Params struct {
	Model     string `json:"model"`
	Language  string `json:"language"`
	Translate bool   `json:"translate"`
}

type Result struct {
	Language string `json:"language"`
}

type TranscriptionItem struct {
	Timestamps Timestamps `json:"timestamps"`
	Offsets    Offsets    `json:"offsets"`
	Text       string     `json:"text"`
	Tokens     []Token    `json:"tokens"`
}

type Timestamps struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Offsets struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type Token struct {
	Text       string     `json:"text"`
	Timestamps Timestamps `json:"timestamps"`
	Offsets    Offsets    `json:"offsets"`
	ID         int        `json:"id"`
	P          float64    `json:"p"`
	TDTW       int        `json:"t_dtw"`
}

func NewClient(opts Options) Client {
	return Client{BinaryPath: opts.BinaryPath, ModelPath: opts.ModelPath}
}

func (c Client) Transcribe(ctx context.Context, opts TranscribeOptions) (*TranscriptionResponse, error) {
	if opts.AudioPath == "" {
		return nil, errors.New("audio path is required")
	}
	if _, err := os.Stat(opts.AudioPath); err != nil {
		return nil, fmt.Errorf("audio file: %w", err)
	}

	modelPath := c.ModelPath
	if modelPath == "" {
		return nil, errors.New("whisper.cpp model path is required")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("whisper.cpp model file: %w", err)
	}

	binaryPath := c.BinaryPath
	if binaryPath == "" {
		binaryPath = defaultBinaryPath
	}
	resolvedBinary, err := exec.LookPath(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("whisper.cpp executable not found: %w", err)
	}

	outputDir, err := os.MkdirTemp("", "trimia-whispercpp-*")
	if err != nil {
		return nil, fmt.Errorf("create whisper.cpp output directory: %w", err)
	}
	defer os.RemoveAll(outputDir)

	outputPrefix := filepath.Join(outputDir, "transcription")
	args := transcribeArgs(modelPath, opts, outputPrefix)
	cmd := exec.CommandContext(ctx, resolvedBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return nil, fmt.Errorf("run whisper.cpp: %w", err)
		}
		return nil, fmt.Errorf("run whisper.cpp: %w: %s", err, message)
	}

	contents, err := os.ReadFile(outputPrefix + ".json")
	if err != nil {
		return nil, fmt.Errorf("read whisper.cpp json output: %w", err)
	}

	var response TranscriptionResponse
	if err := json.Unmarshal(contents, &response); err != nil {
		return nil, fmt.Errorf("decode whisper.cpp json output: %w", err)
	}

	return &response, nil
}

func transcribeArgs(modelPath string, opts TranscribeOptions, outputPrefix string) []string {
	language := opts.Language
	if opts.DetectLanguage || language == "" {
		language = "auto"
	}

	return []string{
		"-m", modelPath,
		"-f", opts.AudioPath,
		"-l", language,
		"-oj",
		"-ojf",
		"-of", outputPrefix,
		"-np",
	}
}
