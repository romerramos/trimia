package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type LogFormat string

const (
	LogFormatHuman LogFormat = "human"
	LogFormatJSON  LogFormat = "json"
)

type logger struct {
	format LogFormat
	out    io.Writer
	std    *log.Logger
}

func newLogger(format LogFormat, out io.Writer) *logger {
	if out == nil {
		out = os.Stdout
	}
	if format != LogFormatJSON {
		format = LogFormatHuman
	}

	return &logger{
		format: format,
		out:    out,
		std:    log.New(out, "", 0),
	}
}

func ParseLogFormat(value string) (LogFormat, error) {
	switch LogFormat(strings.ToLower(value)) {
	case "", LogFormatHuman:
		return LogFormatHuman, nil
	case LogFormatJSON:
		return LogFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported log format %q", value)
	}
}

func (l *logger) Request(method string, path string, status int, duration time.Duration) {
	if l.format == LogFormatJSON {
		l.json("request", map[string]any{
			"method":      method,
			"path":        path,
			"status":      status,
			"duration_ms": duration.Milliseconds(),
		})
		return
	}

	l.std.Printf("%s [%s] %s -> %d (%s)", timestamp(), method, path, status, duration.Round(time.Millisecond))
}

func (l *logger) UploadRejected(reason string) {
	if l.format == LogFormatJSON {
		l.json("upload_rejected", map[string]any{"reason": reason})
		return
	}

	l.std.Printf("%s [WARN] Upload rejected: %s", timestamp(), reason)
}

func (l *logger) UploadSaved(record *mediaRecord) {
	if l.format == LogFormatJSON {
		l.json("upload_saved", map[string]any{
			"media_id":   record.ID,
			"filename":   record.Filename,
			"size_bytes": record.SizeBytes,
			"path":       record.Path,
		})
		return
	}

	l.std.Printf("%s [INFO] Upload saved payload:", timestamp())
	l.std.Printf("   {media_id:%q filename:%q size_bytes:%d path:%q}", record.ID, record.Filename, record.SizeBytes, record.Path)
}

func (l *logger) json(event string, fields map[string]any) {
	fields["ts"] = time.Now().Format("2006/01/02 15:04:05")
	fields["event"] = event
	data, err := json.Marshal(fields)
	if err != nil {
		l.std.Printf(`{"ts":%q,"event":"log_error","error":%q}`, time.Now().Format("2006/01/02 15:04:05"), err.Error())
		return
	}
	_, _ = fmt.Fprintln(l.out, string(data))
}

func timestamp() string {
	return time.Now().Format("2006/01/02 15:04:05")
}
