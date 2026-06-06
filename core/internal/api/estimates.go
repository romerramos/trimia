package api

import (
	"math"
	"sort"
)

func estimateSegmentDuration(segments []segmentResponse, preRollValue, postRollValue, mergeGapValue *float64) float64 {
	preRoll := floatValue(preRollValue, 0.03)
	postRoll := floatValue(postRollValue, 0.06)
	mergeGap := floatValue(mergeGapValue, 0.12)
	prepared := make([]segmentResponse, 0, len(segments))
	for _, segment := range segments {
		if segment.Included {
			segment.Start = math.Max(0, segment.Start-preRoll)
			segment.End += postRoll
			prepared = append(prepared, segment)
		}
	}
	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].Start < prepared[j].Start
	})

	merged := make([]segmentResponse, 0, len(prepared))
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

	duration := 0.0
	for _, segment := range merged {
		duration += segment.End - segment.Start
	}
	return duration
}

func floatValue(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}
