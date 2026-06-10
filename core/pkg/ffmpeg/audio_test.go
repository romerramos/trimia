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

func TestDetectSilenceArgs(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "audio.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	args, err := detectSilenceArgs(DetectSilenceOptions{AudioPath: audioPath})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"-hide_banner",
		"-nostdin",
		"-i", audioPath,
		"-af", "silencedetect=noise=-45dB:d=0.450",
		"-f", "null",
		"-",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestParseSilenceRanges(t *testing.T) {
	output := `[Parsed_silencedetect_0 @ 0x123] silence_start: 53.570312
[Parsed_silencedetect_0 @ 0x123] silence_end: 60.6495 | silence_duration: 7.079187
[Parsed_silencedetect_0 @ 0x123] silence_start: 61.096188
[Parsed_silencedetect_0 @ 0x123] silence_end: 61.4605 | silence_duration: 0.364312`

	got := parseSilenceRanges(output)
	want := []SilenceRange{{Start: 53.570312, End: 60.6495}, {Start: 61.096188, End: 61.4605}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ranges = %#v, want %#v", got, want)
	}
}
