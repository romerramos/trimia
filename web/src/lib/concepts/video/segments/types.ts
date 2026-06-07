import type * as transcription from '$lib/concepts/video/transcription';

export type Segment = {
	id: string;
	start: number;
	end: number;
	text: string;
	source: string;
	included: boolean;
	words?: transcription.Word[];
};

export type ManualInput = {
	start: number;
	end: number;
	now?: number;
};

export type PreviewRange = {
	id: string;
	start: number;
	end: number;
};
