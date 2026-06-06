package trimia

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/internal/ffmpeg"
)

type fakeAudioExtractor struct {
	opts ffmpeg.ExtractAudioOptions
}

func (f *fakeAudioExtractor) ExtractAudio(_ context.Context, opts ffmpeg.ExtractAudioOptions) error {
	f.opts = opts
	return os.WriteFile(opts.OutputPath, []byte("audio"), 0644)
}

type fakeTranscriber struct {
	opts     deepgram.TranscribeOptions
	response *deepgram.TranscriptionResponse
}

func (f *fakeTranscriber) Transcribe(_ context.Context, opts deepgram.TranscribeOptions) (*deepgram.TranscriptionResponse, error) {
	f.opts = opts
	return f.response, nil
}

type fakeVideoCutter struct {
	opts ffmpeg.CutVideoOptions
}

func (f *fakeVideoCutter) CutVideo(_ context.Context, opts ffmpeg.CutVideoOptions) error {
	f.opts = opts
	return nil
}

type fakeDurationProber struct {
	durations map[string]float64
}

func (f fakeDurationProber) ProbeDuration(_ context.Context, path string) (float64, error) {
	return f.durations[path], nil
}

func TestProcessOrchestratesTrimPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	audioExtractor := &fakeAudioExtractor{}
	transcriber := &fakeTranscriber{response: transcriptionResponseWithWords()}
	videoCutter := &fakeVideoCutter{}

	processor := processor{
		audioExtractor: audioExtractor,
		transcriber:    transcriber,
		videoCutter:    videoCutter,
		durationProber: fakeDurationProber{durations: map[string]float64{
			inputPath:  10,
			outputPath: 6,
		}},
	}

	result, err := processor.Process(context.Background(), ProcessOptions{
		InputPath:      inputPath,
		OutputPath:     outputPath,
		DeepgramAPIKey: "test-key",
		Overwrite:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if audioExtractor.opts.InputPath != inputPath {
		t.Fatalf("audio input = %q, want %q", audioExtractor.opts.InputPath, inputPath)
	}

	if audioExtractor.opts.OutputPath == "" {
		t.Fatal("expected temp audio path")
	}

	if _, err := os.Stat(audioExtractor.opts.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp audio to be removed, stat err = %v", err)
	}

	if transcriber.opts.AudioPath != audioExtractor.opts.OutputPath {
		t.Fatalf("transcribe audio = %q, want %q", transcriber.opts.AudioPath, audioExtractor.opts.OutputPath)
	}

	if !transcriber.opts.DetectLanguage {
		t.Fatal("expected detect language default")
	}

	if !transcriber.opts.FillerWords || !transcriber.opts.Punctuate || !transcriber.opts.Utterances {
		t.Fatalf("unexpected transcription opts: %#v", transcriber.opts)
	}

	if videoCutter.opts.InputPath != inputPath || videoCutter.opts.OutputPath != outputPath {
		t.Fatalf("cut opts = %#v", videoCutter.opts)
	}

	if videoCutter.opts.PreRoll != defaultPreRoll || videoCutter.opts.PostRoll != defaultPostRoll || videoCutter.opts.MergeGap != defaultMergeGap {
		t.Fatalf("expected default cut timing opts, got %#v", videoCutter.opts)
	}

	if len(videoCutter.opts.Segments) != 1 || videoCutter.opts.Segments[0].Start != 0.3 || videoCutter.opts.Segments[0].End != 1.2 {
		t.Fatalf("segments = %#v", videoCutter.opts.Segments)
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
	videoCutter := &fakeVideoCutter{}
	processor := processor{
		audioExtractor: &fakeAudioExtractor{},
		transcriber:    &fakeTranscriber{response: transcriptionResponseWithWords()},
		videoCutter:    videoCutter,
		durationProber: fakeDurationProber{durations: map[string]float64{
			inputPath:  10,
			outputPath: 7,
		}},
	}

	_, err := processor.Process(context.Background(), ProcessOptions{
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

	if videoCutter.opts.PreRoll != preRoll || videoCutter.opts.PostRoll != postRoll || videoCutter.opts.MergeGap != mergeGap {
		t.Fatalf("timing opts = %#v", videoCutter.opts)
	}
}

func TestProcessCanKeepTempAudio(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	audioExtractor := &fakeAudioExtractor{}
	processor := processor{
		audioExtractor: audioExtractor,
		transcriber:    &fakeTranscriber{response: transcriptionResponseWithWords()},
		videoCutter:    &fakeVideoCutter{},
		durationProber: fakeDurationProber{durations: map[string]float64{
			inputPath:  10,
			outputPath: 6,
		}},
	}

	result, err := processor.Process(context.Background(), ProcessOptions{
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

	processor := processor{
		audioExtractor: &fakeAudioExtractor{},
		transcriber:    &fakeTranscriber{response: &deepgram.TranscriptionResponse{}},
		videoCutter:    &fakeVideoCutter{},
		durationProber: fakeDurationProber{durations: map[string]float64{
			inputPath: 10,
		}},
	}

	_, err := processor.Process(context.Background(), ProcessOptions{
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
	want := "outputs/demo_trimia.mp4"
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
