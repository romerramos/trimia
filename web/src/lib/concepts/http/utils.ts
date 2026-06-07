import { error } from '@sveltejs/kit';

export async function parseJsonResponse<T>(response: Response, action: string): Promise<T> {
	if (response.ok) {
		return (await response.json()) as T;
	}

	const body = await response.text();
	error(response.status, `Could not ${action}: ${body || response.statusText}`);
}
