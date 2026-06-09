import type * as segments from '$lib/concepts/video/segments';

export function findPreviewIndex(ranges: segments.PreviewRange[], time: number) {
	const containing = ranges.findIndex((range) => time >= range.start && time < range.end);
	if (containing !== -1) {
		return containing;
	}

	const next = ranges.findIndex((range) => range.start > time);
	return next === -1 ? 0 : next;
}

export function playbackStart(ranges: segments.PreviewRange[], currentTime: number) {
	const index = findPreviewIndex(ranges, currentTime);
	const range = ranges[index];

	return {
		index,
		time: currentTime < range.start || currentTime >= range.end ? range.start : currentTime
	};
}

export function nextPlaybackTick(
	ranges: segments.PreviewRange[],
	activeIndex: number,
	currentTime: number
) {
	const range = ranges[activeIndex];
	if (!range) {
		return { action: 'stop' as const };
	}

	if (currentTime < range.end) {
		return { action: 'continue' as const };
	}

	const next = ranges[activeIndex + 1];
	if (!next) {
		return { action: 'finish' as const, time: range.end };
	}

	return { action: 'next' as const, index: activeIndex + 1, time: next.start };
}

export function stopTime(ranges: segments.PreviewRange[]) {
	return ranges[0]?.start;
}
