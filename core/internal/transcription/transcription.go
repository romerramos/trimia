package transcription

import (
	"bytes"
	"context"
	"strings"
)

type Provider string

const (
	ProviderDeepgram Provider = "deepgram"
)

type Transcriber interface {
	Transcribe(context.Context, TranscribeOptions) (*Transcription, error)
}

type TranscribeOptions struct {
	AudioPath      string
	ContentType    string
	Language       string
	DetectLanguage bool
	FillerWords    bool
	Punctuate      bool
	Utterances     bool
}

type Transcription struct {
	Provider           Provider
	OriginalTranscript string
	CleanTranscript    string
	Words              []Word
	FillerWords        []Word
	Segments           []Segment
	Metadata           Metadata
}

type Metadata struct {
	RequestID string
	Duration  float64
	Channels  int
}

type Segment struct {
	Text  string
	Start float64
	End   float64
	Words []Word
}

type Word struct {
	Word           string
	PunctuatedWord string
	Start          float64
	End            float64
	Confidence     float64
	Speaker        int
}

func IsFillerWord(word string) bool {
	switch normalizeWord(word) {
	case "uh", "um", "mhmm", "mm-mm", "uh-uh", "uh-huh", "nuh-uh":
		return true
	default:
		return false
	}
}

func FillerWords(words []Word) []Word {
	fillers := make([]Word, 0)
	for _, word := range words {
		if IsFillerWord(word.Word) {
			fillers = append(fillers, word)
		}
	}

	return fillers
}

func NonFillerWords(words []Word) []Word {
	clean := make([]Word, 0, len(words))
	for _, word := range words {
		if !IsFillerWord(word.Word) {
			clean = append(clean, word)
		}
	}

	return clean
}

func JoinWords(words []Word) string {
	var buffer bytes.Buffer
	for i, word := range words {
		text := word.PunctuatedWord
		if text == "" {
			text = word.Word
		}

		if i > 0 && !isClosingPunctuation(text) {
			buffer.WriteByte(' ')
		}

		buffer.WriteString(text)
	}

	return strings.TrimSpace(buffer.String())
}

func ProviderOrDefault(provider Provider) Provider {
	if provider == "" {
		return ProviderDeepgram
	}

	return provider
}

func isClosingPunctuation(text string) bool {
	return text == "." || text == "," || text == "!" || text == "?" || text == ";" || text == ":"
}

func normalizeWord(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	word = strings.Trim(word, ".,!?;:\"'()[]{}")
	return word
}
