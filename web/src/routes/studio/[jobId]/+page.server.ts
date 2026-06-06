import { redirect } from '@sveltejs/kit';
import { fetchTrimiaJob, fetchTrimiaSegments, trimiaApiUrl, trimiaSourceUrl } from '$lib/server/trimia-api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async (event) => {
	if (!event.locals.user) {
		return redirect(302, '/login');
	}

	const job = await fetchTrimiaJob(event.params.jobId, event.fetch);
	const segments = job.analysis ? await fetchTrimiaSegments(job.jobId, event.fetch) : undefined;

	return {
		user: event.locals.user,
		job,
		segments,
		sourceUrl: trimiaSourceUrl(job.mediaId),
		segmentsUrl: new URL(`/api/jobs/${job.jobId}/segments`, trimiaApiUrl).toString()
	};
};
