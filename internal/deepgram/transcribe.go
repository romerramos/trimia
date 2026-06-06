package deepgram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	defaultModel       = "nova-3"
	defaultContentType = "audio/mp3"
)

type TranscribeOptions struct {
	AudioPath      string
	ContentType    string
	Model          string
	Language       string
	DetectLanguage bool
	FillerWords    bool
	Punctuate      bool
	Utterances     bool
}

type TranscriptionResponse struct {
	Metadata Metadata        `json:"metadata"`
	Results  Results         `json:"results"`
	Raw      json.RawMessage `json:"-"`
}

type Metadata struct {
	RequestID string  `json:"request_id"`
	Duration  float64 `json:"duration"`
	Channels  int     `json:"channels"`
}

type Results struct {
	Channels   []Channel   `json:"channels"`
	Utterances []Utterance `json:"utterances"`
}

type Channel struct {
	Alternatives []Alternative `json:"alternatives"`
}

type Alternative struct {
	Transcript string  `json:"transcript"`
	Confidence float64 `json:"confidence"`
	Words      []Word  `json:"words"`
}

type Word struct {
	Word           string  `json:"word"`
	PunctuatedWord string  `json:"punctuated_word"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Confidence     float64 `json:"confidence"`
	Speaker        int     `json:"speaker,omitempty"`
}

type Utterance struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
	Channel    int     `json:"channel"`
	Transcript string  `json:"transcript"`
	Words      []Word  `json:"words"`
	Speaker    int     `json:"speaker,omitempty"`
	ID         string  `json:"id"`
}

type CleanSegment struct {
	Text  string
	Start float64
	End   float64
	Words []Word
}

func (c *Client) Transcribe(ctx context.Context, opts TranscribeOptions) (*TranscriptionResponse, error) {
	if c == nil {
		return nil, errors.New("deepgram client is required")
	}

	if c.APIKey == "" {
		return nil, errors.New("deepgram api key is required")
	}

	if opts.AudioPath == "" {
		return nil, errors.New("audio path is required")
	}

	audio, err := os.Open(opts.AudioPath)
	if err != nil {
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer audio.Close()

	endpoint, err := c.transcribeURL(opts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, audio)
	if err != nil {
		return nil, fmt.Errorf("create deepgram request: %w", err)
	}

	contentType := opts.ContentType
	if contentType == "" {
		contentType = defaultContentType
	}

	req.Header.Set("Authorization", "Token "+c.APIKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("send deepgram request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read deepgram response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("deepgram request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result TranscriptionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode deepgram response: %w", err)
	}

	result.Raw = append(json.RawMessage(nil), body...)
	return &result, nil
}

func (c *Client) transcribeURL(opts TranscribeOptions) (string, error) {
	endpoint, err := url.Parse(c.baseURL() + "/v1/listen")
	if err != nil {
		return "", fmt.Errorf("parse deepgram url: %w", err)
	}

	model := opts.Model
	if model == "" {
		model = defaultModel
	}

	query := endpoint.Query()
	query.Set("model", model)
	if opts.DetectLanguage {
		query.Set("detect_language", "true")
	} else if opts.Language != "" {
		query.Set("language", opts.Language)
	}
	query.Set("filler_words", boolQueryValue(opts.FillerWords))
	query.Set("punctuate", boolQueryValue(opts.Punctuate))
	query.Set("utterances", boolQueryValue(opts.Utterances))
	endpoint.RawQuery = query.Encode()

	return endpoint.String(), nil
}

func (r *TranscriptionResponse) Transcript() string {
	if r == nil || len(r.Results.Channels) == 0 || len(r.Results.Channels[0].Alternatives) == 0 {
		return ""
	}

	return r.Results.Channels[0].Alternatives[0].Transcript
}

func (r *TranscriptionResponse) Words() []Word {
	if r == nil || len(r.Results.Channels) == 0 || len(r.Results.Channels[0].Alternatives) == 0 {
		return nil
	}

	return r.Results.Channels[0].Alternatives[0].Words
}

func (r *TranscriptionResponse) FillerWords() []Word {
	words := r.Words()
	fillers := make([]Word, 0)
	for _, word := range words {
		if IsFillerWord(word.Word) {
			fillers = append(fillers, word)
		}
	}

	return fillers
}

func (r *TranscriptionResponse) NonFillerWords() []Word {
	words := r.Words()
	clean := make([]Word, 0, len(words))
	for _, word := range words {
		if !IsFillerWord(word.Word) {
			clean = append(clean, word)
		}
	}

	return clean
}

func (r *TranscriptionResponse) CleanTranscript() string {
	return joinWords(r.NonFillerWords())
}

func (r *TranscriptionResponse) CleanSegments() []CleanSegment {
	if r == nil {
		return nil
	}

	if len(r.Results.Utterances) == 0 {
		words := r.NonFillerWords()
		if len(words) == 0 {
			return nil
		}

		return []CleanSegment{{
			Text:  joinWords(words),
			Start: words[0].Start,
			End:   words[len(words)-1].End,
			Words: words,
		}}
	}

	segments := make([]CleanSegment, 0, len(r.Results.Utterances))
	for _, utterance := range r.Results.Utterances {
		words := removeFillerWords(utterance.Words)
		if len(words) == 0 {
			continue
		}

		segments = append(segments, CleanSegment{
			Text:  joinWords(words),
			Start: words[0].Start,
			End:   words[len(words)-1].End,
			Words: words,
		})
	}

	return segments
}

func IsFillerWord(word string) bool {
	switch normalizeWord(word) {
	case "uh", "um", "mhmm", "mm-mm", "uh-uh", "uh-huh", "nuh-uh":
		return true
	default:
		return false
	}
}

func removeFillerWords(words []Word) []Word {
	clean := make([]Word, 0, len(words))
	for _, word := range words {
		if !IsFillerWord(word.Word) {
			clean = append(clean, word)
		}
	}

	return clean
}

func joinWords(words []Word) string {
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

func isClosingPunctuation(text string) bool {
	return text == "." || text == "," || text == "!" || text == "?" || text == ";" || text == ":"
}

func normalizeWord(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))
	word = strings.Trim(word, ".,!?;:\"'()[]{}")
	return word
}

func boolQueryValue(value bool) string {
	if value {
		return "true"
	}

	return "false"
}
