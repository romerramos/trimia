package ffmpeg

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrepareSegmentsPadsSortsAndMerges(t *testing.T) {
	segments, err := prepareSegments([]Segment{
		{Start: 3.0, End: 4.0},
		{Start: 1.0, End: 2.0},
		{Start: 2.25, End: 2.5},
	}, 0.1, 0.2, 0.3)
	if err != nil {
		t.Fatal(err)
	}

	want := []Segment{
		{Start: 0.9, End: 4.2},
	}

	if !reflect.DeepEqual(segments, want) {
		t.Fatalf("segments = %#v, want %#v", segments, want)
	}
}

func TestPrepareSegmentsRejectsInvalidSegment(t *testing.T) {
	_, err := prepareSegments([]Segment{{Start: 2, End: 1}}, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildConcatFilterScript(t *testing.T) {
	script := buildConcatFilterScript([]Segment{
		{Start: 1.2, End: 3.4},
		{Start: 5.6, End: 7.8},
	})

	wantParts := []string{
		"[0:v]trim=start=1.200000:end=3.400000,setpts=PTS-STARTPTS[v0];",
		"[0:a]atrim=start=1.200000:end=3.400000,asetpts=PTS-STARTPTS[a0];",
		"[0:v]trim=start=5.600000:end=7.800000,setpts=PTS-STARTPTS[v1];",
		"[0:a]atrim=start=5.600000:end=7.800000,asetpts=PTS-STARTPTS[a1];",
		"[v0][a0][v1][a1]concat=n=2:v=1:a=1[outv][outa]",
	}

	for _, want := range wantParts {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q in:\n%s", want, script)
		}
	}
}

func TestCutVideoArgsDefaultsToHighQualityFastPreset(t *testing.T) {
	args := cutVideoArgs(CutVideoOptions{
		InputPath:  "input.mp4",
		OutputPath: "output.mp4",
		Overwrite:  true,
	}, "/tmp/filter.txt")

	want := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
		"-stats",
		"-y",
		"-i", "input.mp4",
		"-filter_complex_script", "/tmp/filter.txt",
		"-map", "[outv]",
		"-map", "[outa]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "320k",
		"-movflags", "+faststart",
		"output.mp4",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestSegmentArgsDefaultsToHighQualityFastPreset(t *testing.T) {
	args := segmentArgs(CutVideoOptions{
		InputPath: "input.mp4",
	}, Segment{Start: 1.25, End: 3.5}, "/tmp/segment.mp4")

	want := []string{
		"-hide_banner",
		"-nostdin",
		"-progress", "pipe:1",
		"-stats",
		"-y",
		"-ss", "1.250000",
		"-i", "input.mp4",
		"-t", "2.250000",
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "320k",
		"/tmp/segment.mp4",
	}

	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestSegmentArgsUsesDurationNotAbsoluteEndTime(t *testing.T) {
	args := segmentArgs(CutVideoOptions{InputPath: "input.mp4"}, Segment{Start: 10, End: 13.25}, "segment.mp4")

	if !containsArgPair(args, "-ss", "10.000000") {
		t.Fatalf("args missing seek start: %#v", args)
	}

	if !containsArgPair(args, "-t", "3.250000") {
		t.Fatalf("args missing segment duration: %#v", args)
	}

	if containsArg(args, "-to") {
		t.Fatalf("args should not use absolute end time for segment cuts: %#v", args)
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}

	return false
}

func containsArgPair(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}

	return false
}
