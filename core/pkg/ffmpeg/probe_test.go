package ffmpeg

import (
	"context"
	"testing"
)

func TestProbeDurationRequiresPath(t *testing.T) {
	_, err := ProbeDuration(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}
