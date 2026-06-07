import * as segments from '$lib/concepts/video/segments';
import type { DragDirection, TimelineItem, TimelineZoom } from './types';

export function itemsFromSegments(
	sourceSegments: segments.Segment[],
	duration: number,
	editedSegmentIds: string[]
) {
	if (duration <= 0 || sourceSegments.length === 0) {
		return [] as TimelineItem[];
	}

	const items: TimelineItem[] = [];
	let cursor = 0;

	for (const segment of segments.sortByStart(sourceSegments)) {
		if (segment.start > cursor) {
			items.push({
				id: `gap-${cursor}-${segment.start}`,
				start: cursor,
				end: segment.start,
				included: false,
				kind: 'gap',
				edited: false
			});
		}

		items.push({
			id: segment.id,
			start: segment.start,
			end: segment.end,
			included: segment.included,
			kind: 'segment',
			segmentId: segment.id,
			edited: editedSegmentIds.includes(segment.id) || segment.source === 'manual'
		});
		cursor = Math.max(cursor, segment.end);
	}

	if (cursor < duration) {
		items.push({
			id: `gap-${cursor}-${duration}`,
			start: cursor,
			end: duration,
			included: false,
			kind: 'gap',
			edited: false
		});
	}

	return items.filter((item) => item.end > item.start);
}

export function contextMenuLabel(item: TimelineItem) {
	if (item.kind === 'gap') {
		return 'Add to cut';
	}

	return item.included ? 'Remove from cut' : 'Add to cut';
}

export function itemClass(item: TimelineItem, savingSegmentId: string) {
	return [
		item.included
			? 'bg-foreground hover:bg-foreground/90'
			: 'bg-destructive/25 hover:bg-destructive/35',
		item.edited ? 'opacity-65 ring-1 ring-inset ring-primary/50' : '',
		savingSegmentId === item.segmentId ? 'opacity-50' : '',
		'h-full w-full transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
	].join(' ');
}

export function itemWidth(
	item: TimelineItem,
	duration: number,
	zoom: TimelineZoom,
	pixelsPerSecond: number
) {
	if (zoom === 'playhead') {
		return `${(item.end - item.start) * pixelsPerSecond}px`;
	}

	if (duration <= 0) {
		return '0%';
	}

	return `${((item.end - item.start) / duration) * 100}%`;
}

export function contentWidth(duration: number, zoom: TimelineZoom, pixelsPerSecond: number) {
	return zoom === 'fit' ? '100%' : `${Math.max(duration * pixelsPerSecond, 960)}px`;
}

export function playheadLeft(
	sourceTime: number,
	duration: number,
	zoom: TimelineZoom,
	pixelsPerSecond: number
) {
	if (zoom === 'playhead') {
		return `${sourceTime * pixelsPerSecond}px`;
	}

	const percent = duration > 0 ? Math.min(Math.max((sourceTime / duration) * 100, 0), 100) : 0;
	return `${percent}%`;
}

export function resolveAcceptedDragTime(
	ranges: segments.PreviewRange[],
	time: number,
	direction: DragDirection
) {
	const containing = ranges.find((range) => time >= range.start && time <= range.end);
	if (containing) {
		return time;
	}

	if (direction === 'forward') {
		const next = ranges.find((range) => range.start > time);
		return next?.start ?? ranges.at(-1)?.end ?? 0;
	}

	const previous = ranges.toReversed().find((range) => range.end < time);
	return previous?.end ?? ranges[0]?.start ?? 0;
}
