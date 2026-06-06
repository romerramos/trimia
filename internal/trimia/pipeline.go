package trimia

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"romerramos/trimia/internal/deepgram"
	"romerramos/trimia/pkg/ffmpeg"
)

type mediaProcessor interface {
	ExtractAudio(context.Context, ffmpeg.ExtractAudioOptions) error
	CutVideo(context.Context, ffmpeg.CutVideoOptions) error
	ProbeDuration(context.Context, string) (float64, error)
}

type transcriber interface {
	Transcribe(context.Context, deepgram.TranscribeOptions) (*deepgram.TranscriptionResponse, error)
}

type pipeline struct {
	mediaProcessor mediaProcessor
	transcriber    transcriber
	logWriter      io.Writer
	logger         *log.Logger
	Progress       ProgressFunc
}

func newPipeline(apiKey string) pipeline {
	return pipeline{
		mediaProcessor: ffmpeg.NewClient(),
		transcriber:    deepgram.NewClient(apiKey),
	}
}

func (p pipeline) Run(ctx context.Context, opts ProcessOptions) (*ProcessResult, error) {
	if err := validateProcessOptions(opts); err != nil {
		return nil, err
	}

	logFile, logPath, err := createRunLog(opts.LogDir)
	if err != nil {
		return nil, err
	}
	if logFile != nil {
		defer logFile.Close()
		p.logWriter = logFile
		p.logger = log.New(logFile, "", log.LstdFlags)
		fmt.Printf("Logging to %s\n", logPath)
		fmt.Printf("Run: tail -f %s\n", logPath)
	}
	p.Progress = opts.Progress

	opts = applyDefaults(opts)
	preRoll := valueOrDefault(opts.PreRoll, defaultPreRoll)
	postRoll := valueOrDefault(opts.PostRoll, defaultPostRoll)
	mergeGap := valueOrDefault(opts.MergeGap, defaultMergeGap)

	p.logf("process: input=%s output=%s", opts.InputPath, opts.OutputPath)
	p.logf("process: options pre_roll=%.3f post_roll=%.3f merge_gap=%.3f detect_language=%t language=%q keep_temp_files=%t", preRoll, postRoll, mergeGap, opts.DetectLanguage, opts.Language, opts.KeepTempFiles)
	p.logf("probe input: starting")
	inputDuration, err := p.mediaProcessor.ProbeDuration(ctx, opts.InputPath)
	if err != nil {
		return nil, err
	}
	p.logf("probe input: completed duration=%.2fs", inputDuration)

	audioPath, err := createTempAudioPath()
	if err != nil {
		return nil, err
	}

	if !opts.KeepTempFiles {
		defer os.Remove(audioPath)
	}

	p.logf("extract audio: starting output=%s", audioPath)
	p.progress("Extracting audio", 0)
	if err := p.mediaProcessor.ExtractAudio(ctx, ffmpeg.ExtractAudioOptions{
		InputPath:  opts.InputPath,
		OutputPath: audioPath,
		Overwrite:  true,
		LogWriter:  p.logWriter,
		Progress:   p.phaseProgress("Extracting audio"),
		Duration:   inputDuration,
	}); err != nil {
		return nil, err
	}
	p.logf("extract audio: completed")
	p.progress("Extracting audio", 100)

	p.logf("transcribe: starting Deepgram request")
	stopTranscribeProgress := p.startIndeterminateProgress("Transcribing with Deepgram")
	transcription, err := p.transcriber.Transcribe(ctx, deepgram.TranscribeOptions{
		AudioPath:      audioPath,
		ContentType:    "audio/mp3",
		Model:          "nova-3",
		Language:       opts.Language,
		DetectLanguage: opts.DetectLanguage,
		FillerWords:    opts.RemoveFillerWords,
		Punctuate:      true,
		Utterances:     true,
	})
	stopTranscribeProgress()
	if err != nil {
		return nil, err
	}
	p.logf("transcribe: completed request_id=%s duration=%.2fs", transcription.Metadata.RequestID, transcription.Metadata.Duration)

	cleanSegments := transcription.CleanSegments()
	if len(cleanSegments) == 0 {
		return nil, errors.New("no speech segments found")
	}
	p.logf("segments: clean=%d filler_words=%d", len(cleanSegments), len(transcription.FillerWords()))

	segments := toSegments(cleanSegments)
	ffmpegSegments := toFFmpegSegments(segments)

	p.logf("cut video: starting segments=%d", len(ffmpegSegments))
	p.progress("Rendering video", 0)
	if err := p.mediaProcessor.CutVideo(ctx, ffmpeg.CutVideoOptions{
		InputPath:  opts.InputPath,
		OutputPath: opts.OutputPath,
		Segments:   ffmpegSegments,
		Overwrite:  opts.Overwrite,
		PreRoll:    preRoll,
		PostRoll:   postRoll,
		MergeGap:   mergeGap,
		LogWriter:  p.logWriter,
		Progress:   p.phaseProgress("Rendering video"),
		RenderMode: opts.RenderMode,
		Preset:     opts.VideoPreset,
		CRF:        opts.VideoCRF,
		AudioRate:  opts.AudioRate,
	}); err != nil {
		return nil, err
	}
	p.logf("cut video: completed")
	p.progress("Rendering video", 100)

	p.logf("probe output: starting")
	outputDuration, err := p.mediaProcessor.ProbeDuration(ctx, opts.OutputPath)
	if err != nil {
		return nil, err
	}
	p.logf("probe output: completed duration=%.2fs", outputDuration)

	removedSeconds := inputDuration - outputDuration
	removedPercent := 0.0
	if inputDuration > 0 {
		removedPercent = removedSeconds / inputDuration * 100
	}
	p.logf("process: completed removed=%.2fs removed_percent=%.1f output=%s", removedSeconds, removedPercent, opts.OutputPath)

	resultAudioPath := ""
	if opts.KeepTempFiles {
		resultAudioPath = audioPath
	}

	return &ProcessResult{
		InputPath:             opts.InputPath,
		OutputPath:            opts.OutputPath,
		AudioPath:             resultAudioPath,
		OriginalTranscript:    transcription.Transcript(),
		CleanTranscript:       transcription.CleanTranscript(),
		FillerWords:           transcription.FillerWords(),
		Segments:              segments,
		InputDurationSeconds:  inputDuration,
		OutputDurationSeconds: outputDuration,
		RemovedSeconds:        removedSeconds,
		RemovedPercent:        removedPercent,
	}, nil
}

func (p pipeline) logf(format string, args ...any) {
	if p.logger == nil {
		return
	}

	p.logger.Printf(format, args...)
}

func (p pipeline) progress(phase string, percent float64) {
	if p.Progress == nil {
		return
	}

	p.Progress(phase, percent)
}

func (p pipeline) phaseProgress(phase string) ffmpeg.ProgressFunc {
	if p.Progress == nil {
		return nil
	}

	return func(percent float64) {
		p.progress(phase, percent)
	}
}

func (p pipeline) startIndeterminateProgress(phase string) func() {
	if p.Progress == nil {
		return func() {}
	}

	done := make(chan struct{})
	p.progress(phase, -1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.progress(phase, -1)
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
		p.progress(phase, 100)
	}
}

func createRunLog(logDir string) (*os.File, string, error) {
	if logDir == "" {
		return nil, "", nil
	}

	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}

	path := filepath.Join(logDir, "trimia-"+time.Now().Format("20060102-150405")+".log")
	file, err := os.Create(path)
	if err != nil {
		return nil, "", fmt.Errorf("create run log: %w", err)
	}

	return file, path, nil
}
