import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';

export const trimiaApiUrl = env.TRIMIA_API_URL ?? 'http://127.0.0.1:3333';

export type TrimiaJob = {
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

export type TrimiaSegment = {
	id: string;
	start: number;
	end: number;
	text: string;
	source: string;
	included: boolean;
	words?: TrimiaWord[];
};

export type TrimiaWord = {
	word: string;
	punctuatedWord: string;
	start: number;
	end: number;
	confidence: number;
	filler?: boolean;
};

export type TrimiaSegmentsResponse = {
	jobId: string;
	version: number;
	segments: TrimiaSegment[];
};

export type CreateTrimiaJobResponse = TrimiaJob;

export async function fetchTrimiaJob(jobId: string, fetcher: typeof fetch) {
	const response = await fetcher(new URL(`/api/jobs/${jobId}`, trimiaApiUrl));
	return parseTrimiaResponse<TrimiaJob>(response, 'load job');
}

export async function fetchTrimiaSegments(jobId: string, fetcher: typeof fetch) {
	const response = await fetcher(new URL(`/api/jobs/${jobId}/segments`, trimiaApiUrl));
	if (response.status === 409) {
		return undefined;
	}
	return parseTrimiaResponse<TrimiaSegmentsResponse>(response, 'load segments');
}

export async function createTrimiaJob(mediaId: string, fetcher: typeof fetch) {
	const response = await fetcher(new URL('/api/jobs', trimiaApiUrl), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			mediaId,
			options: {
				removeSilence: true,
				removeFillerWords: true,
				detectLanguage: true
			}
		})
	});

	return parseTrimiaResponse<CreateTrimiaJobResponse>(response, 'create job');
}

export function trimiaSourceUrl(mediaId: string) {
	return new URL(`/api/media/${mediaId}/source`, trimiaApiUrl).toString();
}

async function parseTrimiaResponse<T>(response: Response, action: string) {
	if (response.ok) {
		return (await response.json()) as T;
	}

	const body = await response.text();
	error(response.status, `Could not ${action}: ${body || response.statusText}`);
}
