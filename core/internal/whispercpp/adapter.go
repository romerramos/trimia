package whispercpp

import (
	"context"
	"strings"
	"unicode"

	"romerramos/trimia/internal/transcription"
)

const Provider = transcription.ProviderWhisperCPP

type Transcriber struct {
	client Client
}

func NewTranscriber(opts Options) Transcriber {
	return Transcriber{client: NewClient(opts)}
}

func (t Transcriber) Transcribe(ctx context.Context, opts transcription.TranscribeOptions) (*transcription.Transcription, error) {
	response, err := t.client.Transcribe(ctx, TranscribeOptions{
		AudioPath:      opts.AudioPath,
		Language:       opts.Language,
		DetectLanguage: opts.DetectLanguage,
	})
	if err != nil {
		return nil, err
	}

	return ToTranscription(response), nil
}

func ToTranscription(response *TranscriptionResponse) *transcription.Transcription {
	result := &transcription.Transcription{Provider: Provider}
	if response == nil {
		return result
	}

	segments := make([]transcription.Segment, 0, len(response.Transcription))
	allWords := make([]transcription.Word, 0)
	for _, item := range response.Transcription {
		text := strings.TrimSpace(item.Text)
		words := tokensToWords(item.Tokens)
		if text == "" && len(words) == 0 {
			continue
		}

		segment := transcription.Segment{
			Text:  text,
			Start: millisecondsToSeconds(item.Offsets.From),
			End:   millisecondsToSeconds(item.Offsets.To),
			Words: words,
		}
		segments = append(segments, segment)
		allWords = append(allWords, words...)
	}

	result.Words = allWords
	result.FillerWords = transcription.FillerWords(allWords)
	result.OriginalTranscript = strings.TrimSpace(joinSegmentText(segments))
	cleanWords := transcription.NonFillerWords(allWords)
	result.CleanTranscript = transcription.JoinWords(cleanWords)
	result.Segments = cleanSegments(segments)
	return result
}

func tokensToWords(tokens []Token) []transcription.Word {
	words := make([]transcription.Word, 0, len(tokens))
	for _, token := range tokens {
		rawText := token.Text
		text := strings.TrimSpace(rawText)
		if text == "" || isSpecialToken(text) {
			continue
		}

		if isPunctuationOnly(text) {
			if len(words) == 0 {
				continue
			}
			previous := &words[len(words)-1]
			previous.PunctuatedWord += text
			previous.End = millisecondsToSeconds(token.Offsets.To)
			if token.P < previous.Confidence {
				previous.Confidence = token.P
			}
			continue
		}
		if len(words) > 0 && !startsWithSpace(rawText) {
			previous := &words[len(words)-1]
			previous.Word += strings.TrimFunc(text, unicode.IsPunct)
			previous.PunctuatedWord += text
			previous.End = millisecondsToSeconds(token.Offsets.To)
			if token.P < previous.Confidence {
				previous.Confidence = token.P
			}
			continue
		}

		word := transcription.Word{
			Word:           strings.TrimFunc(text, unicode.IsPunct),
			PunctuatedWord: text,
			Start:          millisecondsToSeconds(token.Offsets.From),
			End:            millisecondsToSeconds(token.Offsets.To),
			Confidence:     token.P,
		}
		if word.Word == "" {
			continue
		}
		words = append(words, word)
	}

	return words
}

func startsWithSpace(text string) bool {
	for _, r := range text {
		return unicode.IsSpace(r)
	}
	return false
}

func cleanSegments(segments []transcription.Segment) []transcription.Segment {
	clean := make([]transcription.Segment, 0, len(segments))
	for _, segment := range segments {
		words := transcription.NonFillerWords(segment.Words)
		if len(words) == 0 {
			continue
		}

		clean = append(clean, transcription.Segment{
			Text:  transcription.JoinWords(words),
			Start: words[0].Start,
			End:   words[len(words)-1].End,
			Words: words,
		})
	}
	return clean
}

func joinSegmentText(segments []transcription.Segment) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment.Text != "" {
			parts = append(parts, segment.Text)
		}
	}
	return strings.Join(parts, " ")
}

func isSpecialToken(text string) bool {
	return strings.HasPrefix(text, "[_") && strings.HasSuffix(text, "]")
}

func isPunctuationOnly(text string) bool {
	for _, r := range text {
		if !unicode.IsPunct(r) {
			return false
		}
	}
	return text != ""
}

func millisecondsToSeconds(value int) float64 {
	return float64(value) / 1000
}
