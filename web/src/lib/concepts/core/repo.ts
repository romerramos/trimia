import { env } from '$env/dynamic/private';
import { parseJsonResponse } from '$lib/concepts/http';
import type {
	CreateJobResponse,
	Job,
	SaveSegmentsInput,
	SaveSegmentsResponse,
	SegmentsResponse
} from './types';

export const apiUrl = env.TRIMIA_API_URL ?? 'http://127.0.0.1:3333';

export async function fetchJob(jobId: string, fetcher: typeof fetch): Promise<Job> {
	const response = await fetcher(new URL(`/api/jobs/${jobId}`, apiUrl));
	return parseJsonResponse<Job>(response, 'load job');
}

export async function fetchSegments(
	jobId: string,
	fetcher: typeof fetch
): Promise<SegmentsResponse | undefined> {
	const response = await fetcher(new URL(`/api/jobs/${jobId}/segments`, apiUrl));
	if (response.status === 409) {
		return undefined;
	}
	return parseJsonResponse<SegmentsResponse>(response, 'load segments');
}

export async function saveSegments(
	jobId: string,
	input: SaveSegmentsInput,
	fetcher: typeof fetch
): Promise<SaveSegmentsResponse> {
	const response = await fetcher(new URL(`/api/jobs/${jobId}/segments`, apiUrl), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(input)
	});

	return parseJsonResponse<SaveSegmentsResponse>(response, 'save segments');
}

export async function createJob(
	mediaId: string,
	fetcher: typeof fetch
): Promise<CreateJobResponse> {
	const response = await fetcher(new URL('/api/jobs', apiUrl), {
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

	return parseJsonResponse<CreateJobResponse>(response, 'create job');
}

export function sourceUrl(mediaId: string): string {
	return new URL(`/api/media/${mediaId}/source`, apiUrl).toString();
}
