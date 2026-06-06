package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	deepgramEnvVar      = "DEEPGRAM_API_KEY"
	deepgramKeyService  = "trimia"
	deepgramKeyUsername = "deepgram-api-key"
)

func resolveDeepgramAPIKey() (string, error) {
	if key := strings.TrimSpace(os.Getenv(deepgramEnvVar)); key != "" {
		return key, nil
	}

	key, err := loadDeepgramAPIKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return "", err
	}

	key, err = promptDeepgramAPIKey()
	if err != nil {
		return "", err
	}

	if err := saveDeepgramAPIKey(key); err != nil {
		return "", fmt.Errorf("save Deepgram API key: %w", err)
	}

	return key, nil
}

func loadDeepgramAPIKey() (string, error) {
	key, err := keyring.Get(deepgramKeyService, deepgramKeyUsername)
	if err != nil {
		return "", err
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", keyring.ErrNotFound
	}

	return key, nil
}

func saveDeepgramAPIKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("Deepgram API key cannot be empty")
	}

	return keyring.Set(deepgramKeyService, deepgramKeyUsername, key)
}

func promptDeepgramAPIKey() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("Deepgram API key is required. Set %s or run `trimia connect`. Create a Deepgram account and get your first $200 in credits for free: https://deepgram.com/", deepgramEnvVar)
	}

	printDeepgramConnectMessage()
	fmt.Fprint(os.Stderr, "Deepgram API key: ")
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read Deepgram API key: %w", err)
	}

	key := strings.TrimSpace(string(bytes))
	if key == "" {
		return "", errors.New("Deepgram API key cannot be empty")
	}

	return key, nil
}

func printDeepgramConnectMessage() {
	fmt.Fprintln(os.Stderr, "Trimia needs a Deepgram API key for transcription.")
	fmt.Fprintln(os.Stderr, "Create a Deepgram account and get your first $200 in credits for free:")
	fmt.Fprintln(os.Stderr, "https://deepgram.com/")
	fmt.Fprintln(os.Stderr)
}
