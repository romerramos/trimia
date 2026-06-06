package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultVideoCodec  = "libx264"
	defaultAudioCodec  = "aac"
	defaultCRF         = 18
	defaultPreset      = "veryfast"
	defaultAudioRate   = "320k"
	renderModeFilter   = "filter"
	renderModeSegments = "segments"
)

type Segment struct {
	Start float64
	End   float64
}

type CutVideoOptions struct {
	InputPath  string
	OutputPath string
	Segments   []Segment
	Overwrite  bool
	PreRoll    float64
	PostRoll   float64
	MergeGap   float64
	VideoCodec string
	AudioCodec string
	CRF        int
	Preset     string
	AudioRate  string
	RenderMode string
	LogWriter  io.Writer
	Progress   ProgressFunc
}

func CutVideo(ctx context.Context, opts CutVideoOptions) error {
	return NewClient().CutVideo(ctx, opts)
}

func (*Client) CutVideo(ctx context.Context, opts CutVideoOptions) error {
	segments, err := prepareSegments(opts.Segments, opts.PreRoll, opts.PostRoll, opts.MergeGap)
	if err != nil {
		return err
	}

	if err := validateCutVideoOptions(opts); err != nil {
		return err
	}

	if opts.RenderMode == "" || opts.RenderMode == renderModeSegments {
		return cutVideoSegments(ctx, opts, segments)
	}

	if opts.RenderMode != renderModeFilter {
		return fmt.Errorf("unsupported render mode %q", opts.RenderMode)
	}

	filterScript := buildConcatFilterScript(segments)
	filterFile, err := os.CreateTemp("", "trimia-ffmpeg-filter-*.txt")
	if err != nil {
		return fmt.Errorf("create filter script: %w", err)
	}
	filterPath := filterFile.Name()
	defer os.Remove(filterPath)

	if _, err := filterFile.WriteString(filterScript); err != nil {
		filterFile.Close()
		return fmt.Errorf("write filter script: %w", err)
	}

	if err := filterFile.Close(); err != nil {
		return fmt.Errorf("close filter script: %w", err)
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg executable not found in PATH")
	}

	args := cutVideoArgs(opts, filterPath)
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	if opts.LogWriter != nil || opts.Progress != nil {
		if err := runWithProgress(cmd, opts.LogWriter, opts.Progress, totalSegmentDuration(segments)); err != nil {
			return fmt.Errorf("cut video: %w", err)
		}

		return nil
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("cut video: %w", err)
		}

		return fmt.Errorf("cut video: %w: %s", err, message)
	}

	return nil
}

func validateCutVideoOptions(opts CutVideoOptions) error {
	if opts.InputPath == "" {
		return errors.New("input path is required")
	}

	if opts.OutputPath == "" {
		return errors.New("output path is required")
	}

	inputInfo, err := os.Stat(opts.InputPath)
	if err != nil {
		return fmt.Errorf("input file: %w", err)
	}

	if inputInfo.IsDir() {
		return errors.New("input path must be a file")
	}

	if !opts.Overwrite {
		if _, err := os.Stat(opts.OutputPath); err == nil {
			return errors.New("output file already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("output file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	return nil
}

func prepareSegments(segments []Segment, preRoll, postRoll, mergeGap float64) ([]Segment, error) {
	if len(segments) == 0 {
		return nil, errors.New("at least one segment is required")
	}

	prepared := make([]Segment, 0, len(segments))
	for _, segment := range segments {
		if segment.Start < 0 || segment.End < 0 {
			return nil, errors.New("segment times must be greater than or equal to zero")
		}

		if segment.End <= segment.Start {
			return nil, errors.New("segment end must be greater than start")
		}

		start := math.Max(0, segment.Start-preRoll)
		end := segment.End + postRoll
		prepared = append(prepared, Segment{Start: start, End: end})
	}

	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].Start < prepared[j].Start
	})

	merged := make([]Segment, 0, len(prepared))
	for _, segment := range prepared {
		if len(merged) == 0 {
			merged = append(merged, segment)
			continue
		}

		last := &merged[len(merged)-1]
		if segment.Start <= last.End+mergeGap {
			if segment.End > last.End {
				last.End = segment.End
			}
			continue
		}

		merged = append(merged, segment)
	}

	return merged, nil
}

func buildConcatFilterScript(segments []Segment) string {
	var builder strings.Builder
	for i, segment := range segments {
		builder.WriteString(fmt.Sprintf(
			"[0:v]trim=start=%.6f:end=%.6f,setpts=PTS-STARTPTS[v%d];\n",
			segment.Start,
			segment.End,
			i,
		))
		builder.WriteString(fmt.Sprintf(
			"[0:a]atrim=start=%.6f:end=%.6f,asetpts=PTS-STARTPTS[a%d];\n",
			segment.Start,
			segment.End,
			i,
		))
	}

	for i := range segments {
		builder.WriteString(fmt.Sprintf("[v%d][a%d]", i, i))
	}

	builder.WriteString(fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]\n", len(segments)))
	return builder.String()
}

func cutVideoSegments(ctx context.Context, opts CutVideoOptions, segments []Segment) error {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg executable not found in PATH")
	}

	tempDir, err := os.MkdirTemp("", "trimia-segments-*")
	if err != nil {
		return fmt.Errorf("create segments temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	totalDuration := totalSegmentDuration(segments)
	completedDuration := 0.0
	segmentPaths := make([]string, 0, len(segments))
	for i, segment := range segments {
		segmentPath := filepath.Join(tempDir, fmt.Sprintf("segment-%06d.mp4", i))
		segmentPaths = append(segmentPaths, segmentPath)

		progress := ProgressFunc(nil)
		if opts.Progress != nil && totalDuration > 0 {
			progress = func(percent float64) {
				segmentDuration := segment.End - segment.Start
				overall := (completedDuration + segmentDuration*percent/100) / totalDuration * 100
				if overall > 99.9 {
					overall = 99.9
				}
				opts.Progress(overall)
			}
		}

		cmd := exec.CommandContext(ctx, ffmpegPath, segmentArgs(opts, segment, segmentPath)...)
		if err := runWithProgress(cmd, opts.LogWriter, progress, segment.End-segment.Start); err != nil {
			return fmt.Errorf("encode segment %d/%d: %w", i+1, len(segments), err)
		}
		completedDuration += segment.End - segment.Start
	}

	listPath := filepath.Join(tempDir, "concat.txt")
	if err := writeConcatList(listPath, segmentPaths); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, concatArgs(listPath, opts.OutputPath, opts.Overwrite)...)
	if opts.LogWriter != nil {
		cmd.Stdout = opts.LogWriter
		cmd.Stderr = opts.LogWriter
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("concat segments: %w", err)
	}

	if opts.Progress != nil {
		opts.Progress(100)
	}

	return nil
}

func writeConcatList(path string, segmentPaths []string) error {
	var builder strings.Builder
	for _, segmentPath := range segmentPaths {
		builder.WriteString("file '")
		builder.WriteString(strings.ReplaceAll(segmentPath, "'", "'\\''"))
		builder.WriteString("'\n")
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write concat list: %w", err)
	}

	return nil
}

func cutVideoArgs(opts CutVideoOptions, filterPath string) []string {
	settings := cutSettings(opts)

	args := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
		"-stats",
	}
	if opts.Overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}

	args = append(
		args,
		"-i", opts.InputPath,
		"-filter_complex_script", filterPath,
		"-map", "[outv]",
		"-map", "[outa]",
		"-c:v", settings.videoCodec,
		"-preset", settings.preset,
		"-crf", fmt.Sprintf("%d", settings.crf),
		"-pix_fmt", "yuv420p",
		"-c:a", settings.audioCodec,
		"-b:a", settings.audioRate,
		"-movflags", "+faststart",
		opts.OutputPath,
	)

	return args
}

type encodingSettings struct {
	videoCodec string
	audioCodec string
	crf        int
	preset     string
	audioRate  string
}

func cutSettings(opts CutVideoOptions) encodingSettings {
	videoCodec := opts.VideoCodec
	if videoCodec == "" {
		videoCodec = defaultVideoCodec
	}

	audioCodec := opts.AudioCodec
	if audioCodec == "" {
		audioCodec = defaultAudioCodec
	}

	crf := opts.CRF
	if crf == 0 {
		crf = defaultCRF
	}

	preset := opts.Preset
	if preset == "" {
		preset = defaultPreset
	}

	audioRate := opts.AudioRate
	if audioRate == "" {
		audioRate = defaultAudioRate
	}

	return encodingSettings{
		videoCodec: videoCodec,
		audioCodec: audioCodec,
		crf:        crf,
		preset:     preset,
		audioRate:  audioRate,
	}
}

func segmentArgs(opts CutVideoOptions, segment Segment, outputPath string) []string {
	settings := cutSettings(opts)
	duration := segment.End - segment.Start
	args := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
		"-stats",
		"-y",
		"-ss", fmt.Sprintf("%.6f", segment.Start),
		"-i", opts.InputPath,
		"-t", fmt.Sprintf("%.6f", duration),
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", settings.videoCodec,
		"-preset", settings.preset,
		"-crf", fmt.Sprintf("%d", settings.crf),
		"-pix_fmt", "yuv420p",
		"-c:a", settings.audioCodec,
		"-b:a", settings.audioRate,
		outputPath,
	}

	return args
}

func concatArgs(listPath, outputPath string, overwrite bool) []string {
	args := []string{
		"-hide_banner",
		"-nostdin",
	}
	if overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}

	args = append(
		args,
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-movflags", "+faststart",
		outputPath,
	)

	return args
}

func totalSegmentDuration(segments []Segment) float64 {
	total := 0.0
	for _, segment := range segments {
		total += segment.End - segment.Start
	}

	return total
}
