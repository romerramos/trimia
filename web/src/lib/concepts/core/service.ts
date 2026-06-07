import type { Job } from './types';

const readyStatuses = ['awaiting_confirmation', 'completed', 'rendering'];
const processingStatuses = ['queued', 'running'];

export function isReady(job: Job) {
	return readyStatuses.includes(job.status);
}

export function isProcessing(job: Job) {
	return processingStatuses.includes(job.status);
}

export function isIndeterminate(job: Job) {
	return isProcessing(job) && job.progress < 0;
}
