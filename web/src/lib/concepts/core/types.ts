import type * as segments from '$lib/concepts/video/segments';

export type Media = {
	mediaId: string;
	filename: string;
	contentType: string;
	sizeBytes: number;
	durationSeconds: number;
	status: string;
	previewStatus: string;
	previewProgress: number;
	previewError?: string;
	createdAt: string;
};

export type Job = {
	jobId: string;
	mediaId: string;
	status: string;
	phase: string;
	progress: number;
	analysis?: {
		inputDurationSeconds: number;
		estimatedOutputDurationSeconds: number;
		estimatedRemovedSeconds: number;
		estimatedRemovedPercent: number;
		originalTranscript: string;
		cleanTranscript: string;
		segmentsUrl: string;
	};
	error?: string;
	createdAt: string;
	updatedAt: string;
};

export type SegmentsResponse = {
	jobId: string;
	version: number;
	segments: segments.Segment[];
};

export type SaveSegmentsInput = {
	baseVersion: number;
	segments: segments.Segment[];
};

export type SaveSegmentsResponse = {
	version: number;
};

export type CreateJobResponse = Job;
