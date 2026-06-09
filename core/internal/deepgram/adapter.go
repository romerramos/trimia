package deepgram

import (
	"context"

	"romerramos/trimia/internal/transcription"
)

const Provider = transcription.ProviderDeepgram

type Transcriber struct {
	client *Client
}

func NewTranscriber(apiKey string) Transcriber {
	return Transcriber{client: NewClient(apiKey)}
}

func (t Transcriber) Transcribe(ctx context.Context, opts transcription.TranscribeOptions) (*transcription.Transcription, error) {
	response, err := t.client.Transcribe(ctx, TranscribeOptions{
		AudioPath:      opts.AudioPath,
		ContentType:    opts.ContentType,
		Model:          "nova-3",
		Language:       opts.Language,
		DetectLanguage: opts.DetectLanguage,
		FillerWords:    opts.FillerWords,
		Punctuate:      opts.Punctuate,
		Utterances:     opts.Utterances,
	})
	if err != nil {
		return nil, err
	}

	return ToTranscription(response), nil
}

func ToTranscription(response *TranscriptionResponse) *transcription.Transcription {
	words := wordsToTranscriptionWords(response.Words())
	fillerWords := transcription.FillerWords(words)
	cleanWords := transcription.NonFillerWords(words)

	result := &transcription.Transcription{
		Provider:           transcription.ProviderDeepgram,
		OriginalTranscript: response.Transcript(),
		CleanTranscript:    transcription.JoinWords(cleanWords),
		Words:              words,
		FillerWords:        fillerWords,
		Metadata: transcription.Metadata{
			RequestID: response.Metadata.RequestID,
			Duration:  response.Metadata.Duration,
			Channels:  response.Metadata.Channels,
		},
	}

	if len(response.Results.Utterances) == 0 {
		if len(cleanWords) > 0 {
			result.Segments = []transcription.Segment{{
				Text:  transcription.JoinWords(cleanWords),
				Start: cleanWords[0].Start,
				End:   cleanWords[len(cleanWords)-1].End,
				Words: cleanWords,
			}}
		}
		return result
	}

	segments := make([]transcription.Segment, 0, len(response.Results.Utterances))
	for _, utterance := range response.Results.Utterances {
		segmentWords := transcription.NonFillerWords(wordsToTranscriptionWords(utterance.Words))
		if len(segmentWords) == 0 {
			continue
		}

		segments = append(segments, transcription.Segment{
			Text:  transcription.JoinWords(segmentWords),
			Start: segmentWords[0].Start,
			End:   segmentWords[len(segmentWords)-1].End,
			Words: segmentWords,
		})
	}
	result.Segments = segments

	return result
}

func wordsToTranscriptionWords(words []Word) []transcription.Word {
	converted := make([]transcription.Word, 0, len(words))
	for _, word := range words {
		converted = append(converted, transcription.Word{
			Word:           word.Word,
			PunctuatedWord: word.PunctuatedWord,
			Start:          word.Start,
			End:            word.End,
			Confidence:     word.Confidence,
			Speaker:        word.Speaker,
		})
	}

	return converted
}
