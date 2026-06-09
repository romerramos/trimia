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

	"romerramos/trimia/internal/transcription"
	"romerramos/trimia/pkg/ffmpeg"
)

type mediaProcessor interface {
	ExtractAudio(context.Context, ffmpeg.ExtractAudioOptions) error
	CutVideo(context.Context, ffmpeg.CutVideoOptions) error
	ProbeDuration(context.Context, string) (float64, error)
}

type pipeline struct {
	mediaProcessor mediaProcessor
	transcriber    transcription.Transcriber
	logWriter      io.Writer
	logger         *log.Logger
	Progress       ProgressFunc
}

func newPipeline(transcriber transcription.Transcriber) pipeline {
	return pipeline{
		mediaProcessor: ffmpeg.NewClient(),
		transcriber:    transcriber,
	}
}

func (p pipeline) Run(ctx context.Context, opts ProcessOptions) (*ProcessResult, error) {
	if err := validateProcessOptions(opts); err != nil {
		return nil, err
	}

	analysis, err := p.Analyze(ctx, processToAnalyzeOptions(opts))
	if err != nil {
		return nil, err
	}

	render, err := p.Render(ctx, processToRenderOptions(opts, analysis.Segments))
	if err != nil {
		return nil, err
	}

	return &ProcessResult{
		InputPath:             opts.InputPath,
		OutputPath:            opts.OutputPath,
		AudioPath:             analysis.AudioPath,
		TranscriberProvider:   analysis.TranscriberProvider,
		OriginalTranscript:    analysis.OriginalTranscript,
		CleanTranscript:       analysis.CleanTranscript,
		FillerWords:           analysis.FillerWords,
		Segments:              analysis.Segments,
		InputDurationSeconds:  render.InputDurationSeconds,
		OutputDurationSeconds: render.OutputDurationSeconds,
		RemovedSeconds:        render.RemovedSeconds,
		RemovedPercent:        render.RemovedPercent,
	}, nil
}

func (p pipeline) Analyze(ctx context.Context, opts AnalyzeOptions) (*AnalysisResult, error) {
	if err := validateAnalyzeOptions(opts); err != nil {
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

	processOpts := applyDefaults(ProcessOptions{
		InputPath:           opts.InputPath,
		Transcriber:         opts.Transcriber,
		TranscriberProvider: opts.TranscriberProvider,
		RemoveSilence:       opts.RemoveSilence,
		RemoveFillerWords:   opts.RemoveFillerWords,
		Language:            opts.Language,
		DetectLanguage:      opts.DetectLanguage,
	})
	preRoll := valueOrDefault(opts.PreRoll, defaultPreRoll)
	postRoll := valueOrDefault(opts.PostRoll, defaultPostRoll)
	mergeGap := valueOrDefault(opts.MergeGap, defaultMergeGap)

	p.logf("analyze: input=%s", opts.InputPath)
	p.logf("analyze: options pre_roll=%.3f post_roll=%.3f merge_gap=%.3f detect_language=%t language=%q keep_temp_files=%t", preRoll, postRoll, mergeGap, processOpts.DetectLanguage, processOpts.Language, opts.KeepTempFiles)
	p.logf("probe input: starting")
	inputDuration, err := p.mediaProcessor.ProbeDuration(ctx, opts.InputPath)
	if err != nil {
		return nil, err
	}
	p.logf("probe input: completed duration=%.2fs", inputDuration)

	audioPath := opts.AudioPath
	usingProvidedAudio := audioPath != ""
	if audioPath == "" {
		var err error
		audioPath, err = createTempAudioPath()
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
	} else {
		p.logf("extract audio: using existing audio=%s", audioPath)
		p.progress("Extracting audio", 100)
	}

	p.logf("transcribe: starting provider=%s", processOpts.TranscriberProvider)
	stopTranscribeProgress := p.startIndeterminateProgress("Transcribing")
	transcript, err := p.transcriber.Transcribe(ctx, transcription.TranscribeOptions{
		AudioPath:      audioPath,
		ContentType:    "audio/mp3",
		Language:       processOpts.Language,
		DetectLanguage: processOpts.DetectLanguage,
		FillerWords:    processOpts.RemoveFillerWords,
		Punctuate:      true,
		Utterances:     true,
	})
	stopTranscribeProgress()
	if err != nil {
		return nil, err
	}
	p.logf("transcribe: completed provider=%s request_id=%s duration=%.2fs", transcript.Provider, transcript.Metadata.RequestID, transcript.Metadata.Duration)

	cleanSegments := transcript.Segments
	if len(cleanSegments) == 0 {
		return nil, errors.New("no speech segments found")
	}
	p.logf("segments: clean=%d filler_words=%d", len(cleanSegments), len(transcript.FillerWords))

	segments := toSegments(cleanSegments)
	estimatedOutputDuration := estimateOutputDuration(segments, preRoll, postRoll, mergeGap)
	estimatedRemovedSeconds := inputDuration - estimatedOutputDuration
	estimatedRemovedPercent := 0.0
	if inputDuration > 0 {
		estimatedRemovedPercent = estimatedRemovedSeconds / inputDuration * 100
	}
	p.logf("analyze: completed estimated_removed=%.2fs estimated_removed_percent=%.1f", estimatedRemovedSeconds, estimatedRemovedPercent)

	resultAudioPath := ""
	if opts.KeepTempFiles || usingProvidedAudio {
		resultAudioPath = audioPath
	}

	return &AnalysisResult{
		InputPath:                      opts.InputPath,
		AudioPath:                      resultAudioPath,
		TranscriberProvider:            transcript.Provider,
		OriginalTranscript:             transcript.OriginalTranscript,
		CleanTranscript:                transcript.CleanTranscript,
		FillerWords:                    transcript.FillerWords,
		Segments:                       segments,
		InputDurationSeconds:           inputDuration,
		EstimatedOutputDurationSeconds: estimatedOutputDuration,
		EstimatedRemovedSeconds:        estimatedRemovedSeconds,
		EstimatedRemovedPercent:        estimatedRemovedPercent,
	}, nil
}

func (p pipeline) Render(ctx context.Context, opts RenderOptions) (*RenderResult, error) {
	if err := validateRenderOptions(opts); err != nil {
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

	preRoll := valueOrDefault(opts.PreRoll, defaultPreRoll)
	postRoll := valueOrDefault(opts.PostRoll, defaultPostRoll)
	mergeGap := valueOrDefault(opts.MergeGap, defaultMergeGap)
	ffmpegSegments := toFFmpegSegments(opts.Segments)

	p.logf("render: input=%s output=%s segments=%d", opts.InputPath, opts.OutputPath, len(ffmpegSegments))
	p.logf("probe input: starting")
	inputDuration, err := p.mediaProcessor.ProbeDuration(ctx, opts.InputPath)
	if err != nil {
		return nil, err
	}
	p.logf("probe input: completed duration=%.2fs", inputDuration)

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
	p.logf("render: completed removed=%.2fs removed_percent=%.1f output=%s", removedSeconds, removedPercent, opts.OutputPath)

	return &RenderResult{
		InputPath:             opts.InputPath,
		OutputPath:            opts.OutputPath,
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
