import type { ManualInput, PreviewRange, Segment } from './types';

const defaultPreviewRangeOptions = {
	preRoll: 0.03,
	postRoll: 0.06,
	mergeGap: 0.12
};

export function sortByStart(segments: Segment[]) {
	return segments.toSorted((a, b) => a.start - b.start);
}

export function included(segments: Segment[]) {
	return segments.filter((segment) => segment.included);
}

export function includedCount(segments: Segment[]) {
	return included(segments).length;
}

export function duration(segments: Segment[]) {
	return Math.max(...segments.map((segment) => segment.end), 0);
}

export function toggleIncluded(segments: Segment[], segmentId: string, nextIncluded: boolean) {
	return segments.map((segment) =>
		segment.id === segmentId ? { ...segment, included: nextIncluded } : segment
	);
}

export function createManual({ start, end, now = Date.now() }: ManualInput): Segment {
	return {
		id: `manual_${now.toString(36)}_${Math.round(start * 1000).toString(36)}`,
		start,
		end,
		text: '',
		source: 'manual',
		included: true,
		words: []
	};
}

export function insert(segments: Segment[], segment: Segment) {
	return sortByStart([...segments, segment]);
}

export function previewRanges(
	segments: Segment[],
	options: { preRoll: number; postRoll: number; mergeGap: number } = defaultPreviewRangeOptions
) {
	const ranges = included(segments)
		.map((segment) => ({
			id: segment.id,
			start: Math.max(0, segment.start - options.preRoll),
			end: segment.end + options.postRoll
		}))
		.toSorted((a, b) => a.start - b.start);

	const merged: PreviewRange[] = [];
	for (const range of ranges) {
		const last = merged.at(-1);
		if (last && range.start <= last.end + options.mergeGap) {
			last.end = Math.max(last.end, range.end);
			last.id = `${last.id}-${range.id}`;
			continue;
		}

		merged.push({ ...range });
	}

	return merged;
}

export function previewDuration(ranges: PreviewRange[]) {
	return ranges.reduce((total, range) => total + range.end - range.start, 0);
}
