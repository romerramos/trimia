package trimia

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/pkg/ffmpeg"
)

type fakeMediaProcessor struct {
	extractOpts ffmpeg.ExtractAudioOptions
	cutOpts     ffmpeg.CutVideoOptions
	durations   map[string]float64
}

func (f *fakeMediaProcessor) ExtractAudio(_ context.Context, opts ffmpeg.ExtractAudioOptions) error {
	f.extractOpts = opts
	return os.WriteFile(opts.OutputPath, []byte("audio"), 0644)
}

func (f *fakeMediaProcessor) CutVideo(_ context.Context, opts ffmpeg.CutVideoOptions) error {
	f.cutOpts = opts
	return nil
}

func (f *fakeMediaProcessor) ProbeDuration(_ context.Context, path string) (float64, error) {
	return f.durations[path], nil
}

type fakeTranscriber struct {
	opts     deepgram.TranscribeOptions
	response *deepgram.TranscriptionResponse
}

func (f *fakeTranscriber) Transcribe(_ context.Context, opts deepgram.TranscribeOptions) (*deepgram.TranscriptionResponse, error) {
	f.opts = opts
	return f.response, nil
}

func TestProcessOrchestratesTrimPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	mediaProcessor := &fakeMediaProcessor{durations: map[string]float64{
		inputPath:  10,
		outputPath: 6,
	}}
	transcriber := &fakeTranscriber{response: transcriptionResponseWithWords()}

	pipeline := pipeline{
		mediaProcessor: mediaProcessor,
		transcriber:    transcriber,
	}

	result, err := pipeline.Run(context.Background(), ProcessOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		DeepgramAPIKey: "test-key",
		Overwrite:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if mediaProcessor.extractOpts.InputPath != inputPath {
		t.Fatalf("audio input = %q, want %q", mediaProcessor.extractOpts.InputPath, inputPath)
	}

	if mediaProcessor.extractOpts.OutputPath == "" {
		t.Fatal("expected temp audio path")
	}

	if _, err := os.Stat(mediaProcessor.extractOpts.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp audio to be removed, stat err = %v", err)
	}

	if transcriber.opts.AudioPath != mediaProcessor.extractOpts.OutputPath {
		t.Fatalf("transcribe audio = %q, want %q", transcriber.opts.AudioPath, mediaProcessor.extractOpts.OutputPath)
	}

	if !transcriber.opts.DetectLanguage {
		t.Fatal("expected detect language default")
	}

	if !transcriber.opts.FillerWords || !transcriber.opts.Punctuate || !transcriber.opts.Utterances {
		t.Fatalf("unexpected transcription opts: %#v", transcriber.opts)
	}

	if mediaProcessor.cutOpts.InputPath != inputPath || mediaProcessor.cutOpts.OutputPath != outputPath {
		t.Fatalf("cut opts = %#v", mediaProcessor.cutOpts)
	}

	if mediaProcessor.cutOpts.PreRoll != defaultPreRoll || mediaProcessor.cutOpts.PostRoll != defaultPostRoll || mediaProcessor.cutOpts.MergeGap != defaultMergeGap {
		t.Fatalf("expected default cut timing opts, got %#v", mediaProcessor.cutOpts)
	}

	if len(mediaProcessor.cutOpts.Segments) != 1 || mediaProcessor.cutOpts.Segments[0].Start != 0.3 || mediaProcessor.cutOpts.Segments[0].End != 1.2 {
		t.Fatalf("segments = %#v", mediaProcessor.cutOpts.Segments)
	}

	if result.OriginalTranscript != "um Hello world." {
		t.Fatalf("original transcript = %q", result.OriginalTranscript)
	}

	if result.CleanTranscript != "Hello world." {
		t.Fatalf("clean transcript = %q", result.CleanTranscript)
	}

	if len(result.FillerWords) != 1 || result.FillerWords[0].Word != "um" {
		t.Fatalf("filler words = %#v", result.FillerWords)
	}

	if len(result.Segments) != 1 || result.Segments[0].Text != "Hello world." {
		t.Fatalf("result segments = %#v", result.Segments)
	}

	if result.InputDurationSeconds != 10 || result.OutputDurationSeconds != 6 || result.RemovedSeconds != 4 || result.RemovedPercent != 40 {
		t.Fatalf("duration metrics = %#v", result)
	}
}

func TestProcessUsesExplicitTimingOptions(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	preRoll := 0.0
	postRoll := 0.04
	mergeGap := 0.08
	mediaProcessor := &fakeMediaProcessor{durations: map[string]float64{
		inputPath:  10,
		outputPath: 7,
	}}
	pipeline := pipeline{
		mediaProcessor: mediaProcessor,
		transcriber:    &fakeTranscriber{response: transcriptionResponseWithWords()},
	}

	_, err := pipeline.Run(context.Background(), ProcessOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		DeepgramAPIKey: "test-key",
		PreRoll:        &preRoll,
		PostRoll:       &postRoll,
		MergeGap:       &mergeGap,
	})
	if err != nil {
		t.Fatal(err)
	}

	if mediaProcessor.cutOpts.PreRoll != preRoll || mediaProcessor.cutOpts.PostRoll != postRoll || mediaProcessor.cutOpts.MergeGap != mergeGap {
		t.Fatalf("timing opts = %#v", mediaProcessor.cutOpts)
	}
}

func TestProcessCanKeepTempAudio(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	mediaProcessor := &fakeMediaProcessor{durations: map[string]float64{
		inputPath:  10,
		outputPath: 6,
	}}
	pipeline := pipeline{
		mediaProcessor: mediaProcessor,
		transcriber:    &fakeTranscriber{response: transcriptionResponseWithWords()},
	}

	result, err := pipeline.Run(context.Background(), ProcessOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		DeepgramAPIKey: "test-key",
		KeepTempFiles:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(result.AudioPath)

	if result.AudioPath == "" {
		t.Fatal("expected kept audio path")
	}

	if _, err := os.Stat(result.AudioPath); err != nil {
		t.Fatalf("expected kept temp audio file: %v", err)
	}
}

func TestProcessRequiresAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Process(context.Background(), ProcessOptions{
		InputPath:  inputPath,
		OutputPath: filepath.Join(tmpDir, "output.mp4"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessFailsWhenNoSpeechSegmentsFound(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline := pipeline{
		mediaProcessor: &fakeMediaProcessor{durations: map[string]float64{
			inputPath: 10,
		}},
		transcriber: &fakeTranscriber{response: &deepgram.TranscriptionResponse{}},
	}

	_, err := pipeline.Run(context.Background(), ProcessOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		DeepgramAPIKey: "test-key",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultOutputPath(t *testing.T) {
	got := DefaultOutputPath("/videos/demo.mp4")
	want := "/videos/demo_trimia.mp4"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func transcriptionResponseWithWords() *deepgram.TranscriptionResponse {
	words := []deepgram.Word{
		{Word: "um", PunctuatedWord: "um", Start: 0.1, End: 0.2, Confidence: 0.99},
		{Word: "hello", PunctuatedWord: "Hello", Start: 0.3, End: 0.7, Confidence: 0.98},
		{Word: "world", PunctuatedWord: "world.", Start: 0.8, End: 1.2, Confidence: 0.97},
	}

	return &deepgram.TranscriptionResponse{
		Results: deepgram.Results{
			Channels: []deepgram.Channel{{
				Alternatives: []deepgram.Alternative{{
					Transcript: "um Hello world.",
					Words:      words,
				}},
			}},
			Utterances: []deepgram.Utterance{{
				Start:      0.1,
				End:        1.2,
				Transcript: "um Hello world.",
				Words:      words,
			}},
		},
	}
}
