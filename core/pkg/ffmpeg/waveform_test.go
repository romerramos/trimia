package ffmpeg

import "testing"

func TestNormalizeAudiowaveformDataMono(t *testing.T) {
	peaks, err := normalizeAudiowaveformData(audiowaveformOutput{
		Channels: 1,
		Data:     []float64{-128, 64, -32, 127},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := [][]float64{{-1, 0.5, -0.25, 0.9921875}}
	if !equalPeaks(peaks, want) {
		t.Fatalf("peaks = %#v, want %#v", peaks, want)
	}
}

func TestNormalizeAudiowaveformDataStereo(t *testing.T) {
	peaks, err := normalizeAudiowaveformData(audiowaveformOutput{
		Channels: 2,
		Data:     []float64{-100, 100, -50, 50, -25, 25, -10, 10},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := [][]float64{{-1, 1, -0.25, 0.25}, {-0.5, 0.5, -0.1, 0.1}}
	if !equalPeaks(peaks, want) {
		t.Fatalf("peaks = %#v, want %#v", peaks, want)
	}
}

func TestNormalizeAudiowaveformDataRejectsInvalidData(t *testing.T) {
	_, err := normalizeAudiowaveformData(audiowaveformOutput{Channels: 2, Data: []float64{1, 2, 3}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func equalPeaks(got, want [][]float64) bool {
	if len(got) != len(want) {
		return false
	}
	for channel := range want {
		if len(got[channel]) != len(want[channel]) {
			return false
		}
		for index := range want[channel] {
			if got[channel][index] != want[channel][index] {
				return false
			}
		}
	}
	return true
}
