package api

import (
	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/internal/trimia"
)

func segmentToResponse(segment trimia.Segment, source string) segmentResponse {
	words := make([]wordResponse, 0, len(segment.Words))
	for _, word := range segment.Words {
		words = append(words, wordToResponse(word))
	}
	return segmentResponse{ID: segment.ID, Start: segment.Start, End: segment.End, Text: segment.Text, Source: source, Included: segment.Included, Words: words}
}

func responseToSegment(segment segmentResponse) trimia.Segment {
	return trimia.Segment{ID: segment.ID, Start: segment.Start, End: segment.End, Text: segment.Text, Included: segment.Included}
}

func wordToResponse(word deepgram.Word) wordResponse {
	return wordResponse{Word: word.Word, PunctuatedWord: word.PunctuatedWord, Start: word.Start, End: word.End, Confidence: word.Confidence, Filler: deepgram.IsFillerWord(word.Word)}
}

func fillerWords(segments []segmentResponse) []wordResponse {
	fillers := make([]wordResponse, 0)
	for _, segment := range segments {
		for _, word := range segment.Words {
			if word.Filler {
				fillers = append(fillers, word)
			}
		}
	}
	return fillers
}

func countIncluded(segments []segmentResponse) int {
	count := 0
	for _, segment := range segments {
		if segment.Included {
			count++
		}
	}
	return count
}
