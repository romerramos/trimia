package ffmpeg

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type ProgressFunc func(percent float64)

func runWithProgress(cmd *exec.Cmd, logWriter io.Writer, progress ProgressFunc, duration float64) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan struct{}, 2)
	go scanProgress(stdout, logWriter, progress, duration, done)
	go copyLog(stderr, logWriter, done)

	err = cmd.Wait()
	<-done
	<-done
	if err != nil {
		return err
	}

	if progress != nil {
		progress(100)
	}

	return nil
}

func scanProgress(reader io.Reader, logWriter io.Writer, progress ProgressFunc, duration float64, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if logWriter != nil {
			fmt.Fprintln(logWriter, line)
		}

		if progress == nil || duration <= 0 || !strings.HasPrefix(line, "out_time_ms=") {
			continue
		}

		value := strings.TrimPrefix(line, "out_time_ms=")
		microseconds, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}

		percent := microseconds / 1_000_000 / duration * 100
		if percent > 99.9 {
			percent = 99.9
		}

		progress(percent)
	}
}

func copyLog(reader io.Reader, logWriter io.Writer, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	if logWriter == nil {
		io.Copy(io.Discard, reader)
		return
	}

	io.Copy(logWriter, reader)
}
