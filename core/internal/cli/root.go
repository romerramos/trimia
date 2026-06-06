package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"romerramos/trimia/internal/api"
	"romerramos/trimia/internal/trimia"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

type options struct {
	inputPath      string
	outputPath     string
	overwrite      bool
	language       string
	detectLanguage bool
	preRoll        float64
	postRoll       float64
	mergeGap       float64
	keepTempFiles  bool
	logDir         string
	renderMode     string
	videoPreset    string
	videoCRF       int
	audioRate      string
}

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	opts := options{}

	cmd := &cobra.Command{
		Use:           "trimia [input]",
		Short:         "Remove silence and filler words from videos",
		Version:       versionString(),
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if cmd.Flags().Changed("input") {
					return fmt.Errorf("use either positional input or --input, not both")
				}
				opts.inputPath = args[0]
			}

			return run(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.inputPath, "input", "", "input video path")
	cmd.Flags().StringVar(&opts.outputPath, "output", "", "output video path")
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", true, "overwrite output file")
	cmd.Flags().StringVar(&opts.language, "language", "", "Deepgram language code; empty enables language detection")
	cmd.Flags().BoolVar(&opts.detectLanguage, "detect-language", true, "ask Deepgram to detect the spoken language")
	cmd.Flags().Float64Var(&opts.preRoll, "pre-roll", 0.03, "seconds to keep before each speech segment")
	cmd.Flags().Float64Var(&opts.postRoll, "post-roll", 0.06, "seconds to keep after each speech segment")
	cmd.Flags().Float64Var(&opts.mergeGap, "merge-gap", 0.12, "merge speech segments separated by this many seconds")
	cmd.Flags().BoolVar(&opts.keepTempFiles, "keep-temp-files", false, "keep intermediate extracted audio file")
	cmd.Flags().StringVar(&opts.logDir, "log-dir", "", "directory for a timestamped run log; try tmp and tail -f the printed file")
	cmd.Flags().StringVar(&opts.renderMode, "render-mode", "segments", "video render mode: segments or filter")
	cmd.Flags().StringVar(&opts.videoPreset, "preset", "veryfast", "x264 preset for rendering; slower presets compress better but take longer")
	cmd.Flags().IntVar(&opts.videoCRF, "crf", 18, "x264 CRF quality; lower is higher quality")
	cmd.Flags().StringVar(&opts.audioRate, "audio-rate", "320k", "output audio bitrate")
	cmd.AddCommand(newConnectCommand())
	cmd.AddCommand(newDisconnectCommand())
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newServeCommand())

	return cmd
}

func newServeCommand() *cobra.Command {
	var addr string
	var dataDir string
	var uploadTokenSecret string
	var allowedOrigin string
	var maxUploadBytes int64
	var logFormat string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the Trimia HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, err := loadLocalEnv()
			if err != nil {
				return err
			}

			apiKey, err := resolveDeepgramAPIKey()
			if err != nil {
				return err
			}

			if uploadTokenSecret == "" {
				uploadTokenSecret = os.Getenv("TRIMIA_UPLOAD_TOKEN_SECRET")
			}
			if allowedOrigin == "" {
				allowedOrigin = os.Getenv("TRIMIA_ALLOWED_ORIGIN")
			}
			if maxUploadBytes == 0 {
				parsed, err := parseOptionalInt64Env("TRIMIA_MAX_UPLOAD_BYTES")
				if err != nil {
					return err
				}
				maxUploadBytes = parsed
			}
			if logFormat == "" {
				logFormat = os.Getenv("TRIMIA_LOG_FORMAT")
			}
			parsedLogFormat, err := api.ParseLogFormat(logFormat)
			if err != nil {
				return err
			}

			server, err := api.NewServer(api.Options{
				DeepgramAPIKey:    apiKey,
				DataDir:           dataDir,
				UploadTokenSecret: uploadTokenSecret,
				AllowedOrigin:     allowedOrigin,
				MaxUploadBytes:    maxUploadBytes,
				LogFormat:         parsedLogFormat,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Trimia API listening on http://%s\n", addr)
			fmt.Fprintf(out, "Env file: %s\n", displayEnvFile(envFile))
			fmt.Fprintf(out, "Data dir: %s\n", server.DataDir())
			fmt.Fprintf(out, "Uploads dir: %s\n", server.UploadsDir())
			fmt.Fprintf(out, "Upload auth: %s\n", enabledText(server.UploadAuthEnabled()))
			fmt.Fprintf(out, "Allowed origin: %s\n", server.AllowedOrigin())
			fmt.Fprintf(out, "Max upload bytes: %d\n", server.MaxUploadBytes())
			fmt.Fprintf(out, "Log format: %s\n", parsedLogFormat)
			return http.ListenAndServe(addr, server.Handler())
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:3333", "HTTP API listen address")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "directory for uploaded and rendered media")
	cmd.Flags().StringVar(&uploadTokenSecret, "upload-token-secret", "", "secret used to validate browser upload tokens; falls back to TRIMIA_UPLOAD_TOKEN_SECRET")
	cmd.Flags().StringVar(&allowedOrigin, "allowed-origin", "", "allowed browser origin for media uploads; falls back to TRIMIA_ALLOWED_ORIGIN")
	cmd.Flags().Int64Var(&maxUploadBytes, "max-upload-bytes", 0, "maximum upload size in bytes; falls back to TRIMIA_MAX_UPLOAD_BYTES or 5 GiB")
	cmd.Flags().StringVar(&logFormat, "log-format", "", "log format: human or json; falls back to TRIMIA_LOG_FORMAT")

	return cmd
}

func loadLocalEnv() (string, error) {
	for _, path := range []string{".env", "../.env"} {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				return "", fmt.Errorf("load %s: %w", path, err)
			}
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("check %s: %w", path, err)
		}
	}

	return "", nil
}

func displayEnvFile(path string) string {
	if path == "" {
		return "not found"
	}
	return path
}

func enabledText(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func parseOptionalInt64Env(name string) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func newConnectCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Save your Deepgram API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				if _, err := loadDeepgramAPIKey(); err == nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "A Deepgram API key is already saved. Use --force to replace it.")
					return nil
				}
			}

			key, err := promptDeepgramAPIKey()
			if err != nil {
				return err
			}
			result, err := saveDeepgramAPIKey(key)
			if err != nil {
				return fmt.Errorf("save Deepgram API key: %w", err)
			}

			printCredentialSaveResult(cmd, result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "replace an existing saved Deepgram API key")

	return cmd
}

func newDisconnectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Remove saved Deepgram API credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			removedKeyring, keyringErr := deleteKeyringDeepgramAPIKey()
			if keyringErr != nil && !isKeyringUnavailable(keyringErr) {
				return fmt.Errorf("delete OS secure store credential: %w", keyringErr)
			}

			removedFallback, fallbackPath, err := deleteFallbackDeepgramAPIKey()
			if err != nil {
				return fmt.Errorf("delete fallback config file: %w", err)
			}

			if removedKeyring {
				fmt.Fprintln(cmd.OutOrStdout(), "Removed Deepgram API key from OS secure store.")
			}
			if removedFallback {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed fallback config file: %s\n", fallbackPath)
			}
			if !removedKeyring && !removedFallback {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved Deepgram API key was found.")
			}
			if keyringErr != nil && isKeyringUnavailable(keyringErr) {
				fmt.Fprintln(cmd.ErrOrStderr(), "OS secure store was unavailable; checked fallback file only.")
			}

			return nil
		},
	}

	return cmd
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show credential storage metadata",
		Run: func(cmd *cobra.Command, args []string) {
			metadata := inspectCredentialMetadata()
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "Credential source: %s\n", metadata.Source)
			fmt.Fprintf(out, "Environment variable: %s\n", metadata.EnvironmentVariable)
			fmt.Fprintf(out, "OS secure store provider: %s\n", metadata.Provider)
			fmt.Fprintf(out, "OS secure store service: %s\n", metadata.Service)
			fmt.Fprintf(out, "OS secure store username: %s\n", metadata.Username)
			if metadata.KeyringAvailable {
				fmt.Fprintln(out, "OS secure store status: available")
			} else {
				fmt.Fprintln(out, "OS secure store status: unavailable")
			}
			if metadata.KeyringError != nil {
				fmt.Fprintf(out, "OS secure store error: %v\n", metadata.KeyringError)
			}
			fmt.Fprintf(out, "Fallback config file: %s\n", metadata.FallbackPath)
			fmt.Fprintln(out, "Fallback config encrypted: no")
			if metadata.FallbackExists {
				fmt.Fprintf(out, "Fallback config permissions: %04o\n", metadata.FallbackMode)
			} else {
				fmt.Fprintln(out, "Fallback config exists: no")
			}
		},
	}

	return cmd
}

func printCredentialSaveResult(cmd *cobra.Command, result saveCredentialResult) {
	switch result.Source {
	case credentialSourceKeyring:
		fmt.Fprintln(cmd.OutOrStdout(), "Deepgram API key saved to OS secure store.")
	case credentialSourceFallback:
		fmt.Fprintf(cmd.OutOrStdout(), "Deepgram API key saved to fallback config file: %s\n", result.FallbackPath)
		fmt.Fprintln(cmd.ErrOrStderr(), "Warning: fallback config is not encrypted. File permissions were set to 0600.")
	default:
		fmt.Fprintln(cmd.OutOrStdout(), "Deepgram API key saved.")
	}
}

func run(ctx context.Context, opts options) error {
	if opts.inputPath == "" {
		return fmt.Errorf("input video path is required")
	}

	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return fmt.Errorf("load .env: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check .env: %w", err)
	}

	apiKey, err := resolveDeepgramAPIKey()
	if err != nil {
		return err
	}

	if opts.outputPath == "" {
		opts.outputPath = trimia.DefaultOutputPath(opts.inputPath)
	}

	progress := newProgressPrinter()
	result, err := trimia.Process(ctx, trimia.ProcessOptions{
		InputPath:         opts.inputPath,
		OutputPath:        opts.outputPath,
		DeepgramAPIKey:    apiKey,
		RemoveSilence:     true,
		RemoveFillerWords: true,
		Language:          opts.language,
		DetectLanguage:    opts.detectLanguage,
		PreRoll:           &opts.preRoll,
		PostRoll:          &opts.postRoll,
		MergeGap:          &opts.mergeGap,
		Overwrite:         opts.overwrite,
		KeepTempFiles:     opts.keepTempFiles,
		LogDir:            opts.logDir,
		Progress:          progress.Update,
		RenderMode:        opts.renderMode,
		VideoPreset:       opts.videoPreset,
		VideoCRF:          opts.videoCRF,
		AudioRate:         opts.audioRate,
	})
	progress.Done()
	if err != nil {
		return err
	}

	fmt.Println("Result:")
	fmt.Printf("Input: %.2fs\n", result.InputDurationSeconds)
	fmt.Printf("Output: %.2fs\n", result.OutputDurationSeconds)
	fmt.Printf("Removed: %.2fs (%.1f%%)\n", result.RemovedSeconds, result.RemovedPercent)
	fmt.Printf("File: %s\n", result.OutputPath)

	return nil
}
