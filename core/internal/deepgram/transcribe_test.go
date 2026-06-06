package deepgram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTranscribe(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := filepath.Join(tmpDir, "audio.mp3")
	audioBytes := []byte("fake mp3 bytes")
	if err := os.WriteFile(audioPath, audioBytes, 0644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/v1/listen" {
			t.Fatalf("path = %s, want /v1/listen", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got != "Token test-key" {
			t.Fatalf("authorization = %q, want %q", got, "Token test-key")
		}

		if got := r.Header.Get("Content-Type"); got != "audio/mp3" {
			t.Fatalf("content-type = %q, want %q", got, "audio/mp3")
		}

		query := r.URL.Query()
		wantQuery := map[string]string{
			"model":           "nova-3",
			"detect_language": "true",
			"filler_words":    "true",
			"punctuate":       "true",
			"utterances":      "true",
		}

		for key, want := range wantQuery {
			if got := query.Get(key); got != want {
				t.Fatalf("query %s = %q, want %q", key, got, want)
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(body, audioBytes) {
			t.Fatalf("body = %#v, want %#v", body, audioBytes)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"metadata": {"request_id": "request-1", "duration": 3.2, "channels": 1},
			"results": {
				"channels": [{
					"alternatives": [{
						"transcript": "um Hello world.",
						"confidence": 0.95,
						"words": [
							{"word": "um", "punctuated_word": "um", "start": 0.10, "end": 0.20, "confidence": 0.99},
							{"word": "hello", "punctuated_word": "Hello", "start": 0.30, "end": 0.70, "confidence": 0.98},
							{"word": "world", "punctuated_word": "world.", "start": 0.80, "end": 1.20, "confidence": 0.97}
						]
					}]
				}],
				"utterances": [{
					"start": 0.10,
					"end": 1.20,
					"confidence": 0.95,
					"channel": 0,
					"transcript": "um Hello world.",
					"words": [
						{"word": "um", "punctuated_word": "um", "start": 0.10, "end": 0.20, "confidence": 0.99},
						{"word": "hello", "punctuated_word": "Hello", "start": 0.30, "end": 0.70, "confidence": 0.98},
						{"word": "world", "punctuated_word": "world.", "start": 0.80, "end": 1.20, "confidence": 0.97}
					]
				}]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	result, err := client.Transcribe(context.Background(), TranscribeOptions{
		AudioPath:      audioPath,
		ContentType:    "audio/mp3",
		Model:          "nova-3",
		DetectLanguage: true,
		FillerWords:    true,
		Punctuate:      true,
		Utterances:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := result.Transcript(); got != "um Hello world." {
		t.Fatalf("transcript = %q, want %q", got, "um Hello world.")
	}

	fillers := result.FillerWords()
	if len(fillers) != 1 || fillers[0].Word != "um" {
		t.Fatalf("fillers = %#v, want one um", fillers)
	}

	if got := result.CleanTranscript(); got != "Hello world." {
		t.Fatalf("clean transcript = %q, want %q", got, "Hello world.")
	}

	segments := result.CleanSegments()
	if len(segments) != 1 {
		t.Fatalf("segments len = %d, want 1", len(segments))
	}

	segment := segments[0]
	if segment.Text != "Hello world." || segment.Start != 0.30 || segment.End != 1.20 {
		t.Fatalf("segment = %#v", segment)
	}
}

func TestIsFillerWord(t *testing.T) {
	for _, word := range []string{"uh", "um", "mhmm", "mm-mm", "uh-uh", "uh-huh", "nuh-uh", "Um,"} {
		if !IsFillerWord(word) {
			t.Fatalf("expected %q to be a filler word", word)
		}
	}

	for _, word := range []string{"hello", "hmm", "like"} {
		if IsFillerWord(word) {
			t.Fatalf("expected %q not to be a filler word", word)
		}
	}
}
