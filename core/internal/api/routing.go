package api

import "strings"

func splitJobPath(path string) (string, string) {
	rest := strings.TrimPrefix(path, "/api/jobs/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func apiPhase(phase string) string {
	switch phase {
	case "Extracting audio":
		return "extracting_audio"
	case "Transcribing with Deepgram":
		return "transcribing"
	case "Rendering video":
		return "rendering_video"
	default:
		return strings.ToLower(strings.ReplaceAll(phase, " ", "_"))
	}
}
