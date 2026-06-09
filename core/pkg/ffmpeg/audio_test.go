package ffmpeg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractAudioArgs(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mov")
	outputPath := filepath.Join(tmpDir, "audio", "output.wav")

	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	args, err := extractAudioArgs(ExtractAudioOptions{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Overwrite:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
		"-y",
		"-i", inputPath,
		"-vn",
		"-map", "a:0",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		outputPath,
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestExtractAudioArgsRequiresInputPath(t *testing.T) {
	_, err := extractAudioArgs(ExtractAudioOptions{OutputPath: "output.wav"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractAudioArgsRefusesExistingOutputWithoutOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mov")
	outputPath := filepath.Join(tmpDir, "output.wav")

	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(outputPath, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := extractAudioArgs(ExtractAudioOptions{
		InputPath:  inputPath,
		OutputPath: outputPath,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
