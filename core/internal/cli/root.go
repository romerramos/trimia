package cli

import (
	"context"
	"fmt"
	"os"

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

	return cmd
}

func newConnectCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Save your Deepgram API key in the OS credential store",
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
			if err := saveDeepgramAPIKey(key); err != nil {
				return fmt.Errorf("save Deepgram API key: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Deepgram API key saved.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "replace an existing saved Deepgram API key")

	return cmd
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
