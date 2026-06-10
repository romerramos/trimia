package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/internal/transcription"
	"romerramos/trimia/internal/whispercpp"

	"github.com/zalando/go-keyring"
)

type resolvedProvider struct {
	Provider    transcription.Provider
	Transcriber transcription.Transcriber
}

func resolveTranscriber() (resolvedProvider, error) {
	provider, err := loadSelectedProvider()
	if err == nil {
		return transcriberForProvider(provider, false)
	}
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return resolvedProvider{}, err
	}

	if inspectWhisperCPPAvailability().Available {
		return transcriberForProvider(transcription.ProviderWhisperCPP, false)
	}
	if inspectDeepgramAvailability().Available {
		return transcriberForProvider(transcription.ProviderDeepgram, false)
	}

	resolved, err := transcriberForProvider(transcription.ProviderDeepgram, true)
	if err != nil {
		return resolvedProvider{}, err
	}
	if _, err := saveSelectedProvider(transcription.ProviderDeepgram); err != nil {
		return resolvedProvider{}, fmt.Errorf("save selected provider: %w", err)
	}
	return resolved, nil
}

func transcriberForProvider(provider transcription.Provider, promptForDeepgram bool) (resolvedProvider, error) {
	switch provider {
	case transcription.ProviderWhisperCPP:
		availability := inspectWhisperCPPAvailability()
		if !availability.Available {
			return resolvedProvider{}, fmt.Errorf("whisper.cpp is not available on this computer/server: %s", availability.Message)
		}
		return resolvedProvider{
			Provider: provider,
			Transcriber: whispercpp.NewTranscriber(whispercpp.Options{
				BinaryPath: strings.TrimSpace(os.Getenv(whisperCPPBinaryEnv)),
				ModelPath:  strings.TrimSpace(os.Getenv(whisperCPPModelEnv)),
			}),
		}, nil
	case transcription.ProviderDeepgram:
		key := strings.TrimSpace(os.Getenv(deepgramEnvVar))
		if key == "" {
			var err error
			if promptForDeepgram {
				key, err = resolveDeepgramAPIKey()
			} else {
				key, err = loadDeepgramAPIKey()
			}
			if err != nil {
				return resolvedProvider{}, err
			}
		}
		return resolvedProvider{Provider: provider, Transcriber: deepgram.NewTranscriber(key)}, nil
	default:
		return resolvedProvider{}, fmt.Errorf("unsupported provider %q", provider)
	}
}
