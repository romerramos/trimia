import { describe, expect, it } from 'vitest';
import { contentWidth, itemWidth, itemsFromSegments, playheadLeft } from './helpers';
import type * as segments from '$lib/concepts/video/segments';

describe('timeline helpers', () => {
	it('keeps fit-mode segment positions on the same percent scale as the playhead', () => {
		const duration = 1200;
		const items = itemsFromSegments(
			[segment('intro', 0, 0.1), segment('selected', 600, 600.1), segment('outro', 1199.9, 1200)],
			duration,
			[]
		);
		const selectedIndex = items.findIndex((item) => item.id === 'selected');
		const selectedLeft = items
			.slice(0, selectedIndex)
			.reduce((total, item) => total + Number.parseFloat(itemWidth(item, duration, 'fit', 48)), 0);

		expect(selectedLeft).toBeCloseTo(Number.parseFloat(playheadLeft(600, duration, 'fit', 48)));
	});

	it('keeps zoom-mode segment positions on the same pixel scale as the playhead', () => {
		const duration = 1200;
		const pixelsPerSecond = 48;
		const items = itemsFromSegments(
			[segment('intro', 0, 0.1), segment('selected', 600, 600.1), segment('outro', 1199.9, 1200)],
			duration,
			[]
		);
		const selectedIndex = items.findIndex((item) => item.id === 'selected');
		const selectedLeft = items
			.slice(0, selectedIndex)
			.reduce(
				(total, item) =>
					total + Number.parseFloat(itemWidth(item, duration, 'playhead', pixelsPerSecond)),
				0
			);

		expect(contentWidth(duration, 'playhead', pixelsPerSecond)).toBe('57600px');
		expect(selectedLeft).toBeCloseTo(
			Number.parseFloat(playheadLeft(600, duration, 'playhead', pixelsPerSecond))
		);
	});
});

function segment(id: string, start: number, end: number): segments.Segment {
	return {
		id,
		start,
		end,
		text: '',
		source: 'transcript',
		included: true,
		words: []
	};
}
