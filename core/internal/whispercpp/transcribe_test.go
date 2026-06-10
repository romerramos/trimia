package whispercpp

import (
	"reflect"
	"testing"

	"romerramos/trimia/internal/transcription"
)

func TestTranscribeArgsUsesAutoLanguageWhenDetecting(t *testing.T) {
	args := transcribeArgs("model.bin", TranscribeOptions{AudioPath: "audio.wav", DetectLanguage: true}, "out/transcript")
	want := []string{"-m", "model.bin", "-f", "audio.wav", "-l", "auto", "-oj", "-ojf", "-of", "out/transcript", "-np"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestToTranscriptionMergesSubwordTokens(t *testing.T) {
	response := &TranscriptionResponse{Transcription: []TranscriptionItem{{
		Offsets: Offsets{From: 2697370, To: 2697900},
		Text:    " módulo.",
		Tokens: []Token{
			{Text: " m", Offsets: Offsets{From: 2697370, To: 2697410}, P: 0.16},
			{Text: "ód", Offsets: Offsets{From: 2697410, To: 2697560}, P: 0.84},
			{Text: "ulo", Offsets: Offsets{From: 2697590, To: 2697770}, P: 0.99},
			{Text: ".", Offsets: Offsets{From: 2697770, To: 2697900}, P: 0.97},
		},
	}}}

	result := ToTranscription(response)
	if len(result.Segments) != 1 || len(result.Segments[0].Words) != 1 {
		t.Fatalf("segments = %#v", result.Segments)
	}
	word := result.Segments[0].Words[0]
	if word.Word != "módulo" || word.PunctuatedWord != "módulo." {
		t.Fatalf("word = %#v", word)
	}
	if word.Start != 2697.37 || word.End != 2697.9 || word.Confidence != 0.16 {
		t.Fatalf("word timing/confidence = %#v", word)
	}
}

func TestToTranscriptionConvertsSegmentsAndWords(t *testing.T) {
	response := &TranscriptionResponse{Transcription: []TranscriptionItem{{
		Offsets: Offsets{From: 0, To: 10320},
		Text:    " vamos a hablar del mindset,",
		Tokens: []Token{
			{Text: "[_BEG_]", Offsets: Offsets{From: 0, To: 0}, P: 0.7},
			{Text: " vamos", Offsets: Offsets{From: 680, To: 680}, P: 0.42},
			{Text: " a", Offsets: Offsets{From: 720, To: 800}, P: 0.96},
			{Text: " hablar", Offsets: Offsets{From: 1280, To: 1620}, P: 0.99},
			{Text: " del", Offsets: Offsets{From: 1620, To: 1900}, P: 0.98},
			{Text: " mindset", Offsets: Offsets{From: 2170, To: 3000}, P: 0.87},
			{Text: ",", Offsets: Offsets{From: 3000, To: 3070}, P: 0.71},
		},
	}}}

	result := ToTranscription(response)
	if result.Provider != transcription.ProviderWhisperCPP {
		t.Fatalf("provider = %q, want %q", result.Provider, transcription.ProviderWhisperCPP)
	}
	if result.OriginalTranscript != "vamos a hablar del mindset," {
		t.Fatalf("original transcript = %q", result.OriginalTranscript)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("segments len = %d, want 1", len(result.Segments))
	}

	segment := result.Segments[0]
	if segment.Start != 0.68 || segment.End != 3.07 {
		t.Fatalf("segment timing = %.2f-%.2f, want 0.68-3.07", segment.Start, segment.End)
	}
	if len(segment.Words) != 5 {
		t.Fatalf("words len = %d, want 5: %#v", len(segment.Words), segment.Words)
	}

	word := segment.Words[4]
	if word.Word != "mindset" || word.PunctuatedWord != "mindset," {
		t.Fatalf("punctuated word = %#v", word)
	}
	if word.Start != 2.17 || word.End != 3.07 || word.Confidence != 0.71 {
		t.Fatalf("merged word timing/confidence = %#v", word)
	}
}

func TestToTranscriptionRemovesKnownFillers(t *testing.T) {
	response := &TranscriptionResponse{Transcription: []TranscriptionItem{{
		Offsets: Offsets{From: 100, To: 1200},
		Text:    " um Hello world.",
		Tokens: []Token{
			{Text: " um", Offsets: Offsets{From: 100, To: 200}, P: 0.99},
			{Text: " Hello", Offsets: Offsets{From: 300, To: 700}, P: 0.98},
			{Text: " world", Offsets: Offsets{From: 800, To: 1100}, P: 0.97},
			{Text: ".", Offsets: Offsets{From: 1100, To: 1200}, P: 0.96},
		},
	}}}

	result := ToTranscription(response)
	if result.CleanTranscript != "Hello world." {
		t.Fatalf("clean transcript = %q, want Hello world.", result.CleanTranscript)
	}
	if len(result.FillerWords) != 1 || result.FillerWords[0].Word != "um" {
		t.Fatalf("filler words = %#v", result.FillerWords)
	}
}
