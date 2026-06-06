package api

type createJobRequest struct {
	MediaID string           `json:"mediaId"`
	Options createJobOptions `json:"options"`
}

type createJobOptions struct {
	RemoveSilence     bool     `json:"removeSilence"`
	RemoveFillerWords bool     `json:"removeFillerWords"`
	Language          string   `json:"language"`
	DetectLanguage    bool     `json:"detectLanguage"`
	PreRoll           *float64 `json:"preRoll"`
	PostRoll          *float64 `json:"postRoll"`
	MergeGap          *float64 `json:"mergeGap"`
}

type analysisResponse struct {
	InputDurationSeconds           float64 `json:"inputDurationSeconds"`
	EstimatedOutputDurationSeconds float64 `json:"estimatedOutputDurationSeconds"`
	EstimatedRemovedSeconds        float64 `json:"estimatedRemovedSeconds"`
	EstimatedRemovedPercent        float64 `json:"estimatedRemovedPercent"`
	OriginalTranscript             string  `json:"originalTranscript"`
	CleanTranscript                string  `json:"cleanTranscript"`
	SegmentsURL                    string  `json:"segmentsUrl"`
}

type segmentsResponse struct {
	JobID       string            `json:"jobId"`
	Version     int               `json:"version"`
	Segments    []segmentResponse `json:"segments"`
	FillerWords []wordResponse    `json:"fillerWords"`
}

type segmentResponse struct {
	ID       string         `json:"id"`
	Start    float64        `json:"start"`
	End      float64        `json:"end"`
	Text     string         `json:"text"`
	Source   string         `json:"source"`
	Included bool           `json:"included"`
	Words    []wordResponse `json:"words,omitempty"`
}

type wordResponse struct {
	Word           string  `json:"word"`
	PunctuatedWord string  `json:"punctuatedWord"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	Confidence     float64 `json:"confidence"`
	Filler         bool    `json:"filler,omitempty"`
}

type updateSegmentsRequest struct {
	BaseVersion int               `json:"baseVersion"`
	Segments    []segmentResponse `json:"segments"`
}

type updateSegmentsResponse struct {
	JobID   string `json:"jobId"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Summary struct {
		IncludedSegments               int     `json:"includedSegments"`
		EstimatedOutputDurationSeconds float64 `json:"estimatedOutputDurationSeconds"`
		EstimatedRemovedSeconds        float64 `json:"estimatedRemovedSeconds"`
		EstimatedRemovedPercent        float64 `json:"estimatedRemovedPercent"`
	} `json:"summary"`
}

type renderRequest struct {
	SegmentVersion int `json:"segmentVersion"`
	Output         struct {
		Filename  string `json:"filename"`
		Overwrite bool   `json:"overwrite"`
	} `json:"output"`
	RenderOptions struct {
		RenderMode string `json:"renderMode"`
		Preset     string `json:"preset"`
		CRF        int    `json:"crf"`
		AudioRate  string `json:"audioRate"`
	} `json:"renderOptions"`
}

type renderResultResponse struct {
	OutputMediaID         string  `json:"outputMediaId"`
	Filename              string  `json:"filename"`
	DownloadURL           string  `json:"downloadUrl"`
	InputDurationSeconds  float64 `json:"inputDurationSeconds"`
	OutputDurationSeconds float64 `json:"outputDurationSeconds"`
	RemovedSeconds        float64 `json:"removedSeconds"`
	RemovedPercent        float64 `json:"removedPercent"`
	path                  string
}
