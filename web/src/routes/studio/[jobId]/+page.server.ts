import { fail, redirect } from '@sveltejs/kit';
import * as core from '$lib/concepts/core';
import type * as videoSegments from '$lib/concepts/video/segments';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async (event) => {
	if (!event.locals.user) {
		return redirect(302, '/login');
	}

	const job = await core.fetchJob(event.params.jobId, event.fetch);
	const media = await core.fetchMedia(job.mediaId, event.fetch);
	const segments = job.analysis ? await core.fetchSegments(job.jobId, event.fetch) : undefined;
	const waveform = await core.fetchWaveform(job.mediaId, event.fetch);

	return {
		user: event.locals.user,
		job,
		media,
		segments,
		waveform,
		sourceUrl: core.previewUrl(job.mediaId)
	};
};

export const actions: Actions = {
	saveSegments: async (event) => {
		if (!event.locals.user) {
			return fail(401, { message: 'You must be logged in to save segments.' });
		}

		const formData = await event.request.formData();
		const baseVersion = Number(formData.get('baseVersion'));
		const segmentsValue = formData.get('segments');

		if (!Number.isFinite(baseVersion) || typeof segmentsValue !== 'string') {
			return fail(400, { message: 'Invalid segment save request.' });
		}

		try {
			const segments = JSON.parse(segmentsValue) as videoSegments.Segment[];
			const result = await core.saveSegments(
				event.params.jobId,
				{ baseVersion, segments },
				event.fetch
			);

			return { version: result.version };
		} catch (error) {
			return fail(500, {
				message: error instanceof Error ? error.message : 'Could not save segment changes.'
			});
		}
	}
};
