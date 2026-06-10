package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"romerramos/trimia/internal/transcription"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	deepgramEnvVar      = "DEEPGRAM_API_KEY"
	whisperCPPBinaryEnv = "TRIMIA_WHISPER_CPP_BINARY"
	whisperCPPModelEnv  = "TRIMIA_WHISPER_CPP_MODEL"
	deepgramKeyService  = "trimia"
	deepgramKeyUsername = "deepgram-api-key"
	configDirName       = ".trimia"
	configFileName      = "config.json"
)

type credentialSource string

const (
	credentialSourceNone     credentialSource = "none"
	credentialSourceEnv      credentialSource = "environment variable"
	credentialSourceKeyring  credentialSource = "OS secure store"
	credentialSourceFallback credentialSource = "fallback file"
)

type credentialMetadata struct {
	Source              credentialSource
	EnvironmentVariable string
	Provider            string
	Service             string
	Username            string
	FallbackPath        string
	FallbackExists      bool
	FallbackMode        os.FileMode
	KeyringAvailable    bool
	KeyringError        error
}

type storedConfig struct {
	SelectedProvider string `json:"selected_provider,omitempty"`
	DeepgramAPIKey   string `json:"deepgram_api_key,omitempty"`
}

type providerAvailability struct {
	Provider  transcription.Provider
	Available bool
	Message   string
}

type saveCredentialResult struct {
	Source       credentialSource
	FallbackPath string
}

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

	if _, err := saveDeepgramAPIKey(key); err != nil {
		return "", fmt.Errorf("save Deepgram API key: %w", err)
	}

	return key, nil
}

func loadDeepgramAPIKey() (string, error) {
	key, err := loadKeyringDeepgramAPIKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) && !isKeyringUnavailable(err) {
		return "", err
	}

	return loadFallbackDeepgramAPIKey()
}

func loadKeyringDeepgramAPIKey() (string, error) {
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

func saveDeepgramAPIKey(key string) (saveCredentialResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return saveCredentialResult{}, errors.New("Deepgram API key cannot be empty")
	}

	err := keyring.Set(deepgramKeyService, deepgramKeyUsername, key)
	if err == nil {
		return saveCredentialResult{Source: credentialSourceKeyring}, nil
	}
	if !isKeyringUnavailable(err) {
		return saveCredentialResult{}, err
	}

	path, err := saveFallbackDeepgramAPIKey(key)
	if err != nil {
		return saveCredentialResult{}, err
	}

	return saveCredentialResult{Source: credentialSourceFallback, FallbackPath: path}, nil
}

func fallbackConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}

	return filepath.Join(home, configDirName, configFileName), nil
}

func loadFallbackConfig() (storedConfig, string, error) {
	path, err := fallbackConfigPath()
	if err != nil {
		return storedConfig{}, "", err
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return storedConfig{}, path, keyring.ErrNotFound
		}
		return storedConfig{}, path, err
	}

	var config storedConfig
	if err := json.Unmarshal(bytes, &config); err != nil {
		return storedConfig{}, path, fmt.Errorf("read %s: %w", path, err)
	}

	return config, path, nil
}

func saveFallbackConfig(config storedConfig) (string, error) {
	path, err := fallbackConfigPath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}

	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	bytes = append(bytes, '\n')

	if err := os.WriteFile(path, bytes, 0o600); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}

	return path, nil
}

func loadSelectedProvider() (transcription.Provider, error) {
	config, _, err := loadFallbackConfig()
	if err != nil {
		return "", err
	}

	provider := transcription.Provider(strings.TrimSpace(config.SelectedProvider))
	if provider == "" {
		return "", keyring.ErrNotFound
	}
	if !isSupportedProvider(provider) {
		return "", fmt.Errorf("unsupported selected provider %q", provider)
	}

	return provider, nil
}

func saveSelectedProvider(provider transcription.Provider) (string, error) {
	if !isSupportedProvider(provider) {
		return "", fmt.Errorf("unsupported provider %q", provider)
	}

	config, _, err := loadFallbackConfig()
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return "", err
	}
	config.SelectedProvider = string(provider)
	return saveFallbackConfig(config)
}

func isSupportedProvider(provider transcription.Provider) bool {
	switch provider {
	case transcription.ProviderWhisperCPP, transcription.ProviderDeepgram:
		return true
	default:
		return false
	}
}

func loadFallbackDeepgramAPIKey() (string, error) {
	config, _, err := loadFallbackConfig()
	if err != nil {
		return "", err
	}

	key := strings.TrimSpace(config.DeepgramAPIKey)
	if key == "" {
		return "", keyring.ErrNotFound
	}

	return key, nil
}

func saveFallbackDeepgramAPIKey(key string) (string, error) {
	config, _, err := loadFallbackConfig()
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return "", err
	}
	config.DeepgramAPIKey = key

	return saveFallbackConfig(config)
}

func deleteFallbackDeepgramAPIKey() (bool, string, error) {
	path, err := fallbackConfigPath()
	if err != nil {
		return false, "", err
	}

	config, _, err := loadFallbackConfig()
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return false, path, nil
		}
		return false, path, err
	}
	if strings.TrimSpace(config.DeepgramAPIKey) == "" {
		return false, path, nil
	}
	config.DeepgramAPIKey = ""

	if strings.TrimSpace(config.SelectedProvider) == "" {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return false, path, nil
			}
			return false, path, err
		}
		return true, path, nil
	}

	if _, err := saveFallbackConfig(config); err != nil {
		if os.IsNotExist(err) {
			return false, path, nil
		}
		return false, path, err
	}

	return true, path, nil
}

func inspectProviderAvailability() []providerAvailability {
	return []providerAvailability{inspectWhisperCPPAvailability(), inspectDeepgramAvailability()}
}

func inspectWhisperCPPAvailability() providerAvailability {
	binaryPath := strings.TrimSpace(os.Getenv(whisperCPPBinaryEnv))
	if binaryPath == "" {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: fmt.Sprintf("%s is not set", whisperCPPBinaryEnv)}
	}
	resolvedBinary, err := exec.LookPath(binaryPath)
	if err != nil {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: "whisper.cpp is not available on this computer/server: binary not found"}
	}
	info, err := os.Stat(resolvedBinary)
	if err != nil {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: fmt.Sprintf("whisper.cpp binary: %v", err)}
	}
	if info.IsDir() {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: "whisper.cpp binary path is a directory"}
	}
	if info.Mode().Perm()&0o111 == 0 {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: "whisper.cpp binary is not executable"}
	}

	modelPath := strings.TrimSpace(os.Getenv(whisperCPPModelEnv))
	if modelPath == "" {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: fmt.Sprintf("%s is not set", whisperCPPModelEnv)}
	}
	modelInfo, err := os.Stat(modelPath)
	if err != nil {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: fmt.Sprintf("whisper.cpp model file: %v", err)}
	}
	if modelInfo.IsDir() {
		return providerAvailability{Provider: transcription.ProviderWhisperCPP, Message: "whisper.cpp model path is a directory"}
	}

	return providerAvailability{Provider: transcription.ProviderWhisperCPP, Available: true, Message: "available"}
}

func inspectDeepgramAvailability() providerAvailability {
	if strings.TrimSpace(os.Getenv(deepgramEnvVar)) != "" {
		return providerAvailability{Provider: transcription.ProviderDeepgram, Available: true, Message: fmt.Sprintf("available through %s", deepgramEnvVar)}
	}
	if _, err := loadDeepgramAPIKey(); err == nil {
		return providerAvailability{Provider: transcription.ProviderDeepgram, Available: true, Message: "available through saved API key"}
	} else if !errors.Is(err, keyring.ErrNotFound) {
		return providerAvailability{Provider: transcription.ProviderDeepgram, Message: err.Error()}
	}

	return providerAvailability{Provider: transcription.ProviderDeepgram, Message: "Deepgram API key is not configured"}
}

func deleteKeyringDeepgramAPIKey() (bool, error) {
	err := keyring.Delete(deepgramKeyService, deepgramKeyUsername)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	if isKeyringUnavailable(err) {
		return false, err
	}

	return false, err
}

func inspectCredentialMetadata() credentialMetadata {
	metadata := credentialMetadata{
		Source:              credentialSourceNone,
		EnvironmentVariable: deepgramEnvVar,
		Provider:            keyringProviderName(),
		Service:             deepgramKeyService,
		Username:            deepgramKeyUsername,
	}
	if path, err := fallbackConfigPath(); err == nil {
		metadata.FallbackPath = path
	} else {
		metadata.KeyringError = err
	}

	if strings.TrimSpace(os.Getenv(deepgramEnvVar)) != "" {
		metadata.Source = credentialSourceEnv
		return metadata
	}

	if _, err := loadKeyringDeepgramAPIKey(); err == nil {
		metadata.Source = credentialSourceKeyring
		metadata.KeyringAvailable = true
		return metadata
	} else if isKeyringUnavailable(err) {
		metadata.KeyringError = err
	} else if !errors.Is(err, keyring.ErrNotFound) {
		metadata.KeyringAvailable = true
		metadata.KeyringError = err
	} else {
		metadata.KeyringAvailable = true
	}

	if metadata.FallbackPath == "" {
		return metadata
	}

	info, err := os.Stat(metadata.FallbackPath)
	if err == nil {
		metadata.FallbackExists = true
		metadata.FallbackMode = info.Mode().Perm()
		if _, loadErr := loadFallbackDeepgramAPIKey(); loadErr == nil {
			metadata.Source = credentialSourceFallback
		}
	}

	return metadata
}

func isKeyringUnavailable(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "org.freedesktop.secrets") ||
		strings.Contains(message, "No such interface") ||
		strings.Contains(message, "The name is not activatable")
}

func keyringProviderName() string {
	return "Linux Secret Service, macOS Keychain, or Windows Credential Manager"
}

func promptDeepgramAPIKey() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("Trimia needs a Deepgram API key to process videos. Create a Deepgram account, then run `trimia connect` to save your key. For scripts or CI, set %s. Learn more: https://deepgram.com/", deepgramEnvVar)
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
