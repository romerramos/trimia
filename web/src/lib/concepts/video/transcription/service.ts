type TranscriptSource = {
	cleanTranscript?: string;
	originalTranscript?: string;
};

export function textForDisplay(
	source: TranscriptSource | undefined,
	fallback = 'Transcript will appear when analysis completes.'
) {
	return source?.cleanTranscript || source?.originalTranscript || fallback;
}
