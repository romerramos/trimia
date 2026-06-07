export type TimelineZoom = 'fit' | 'playhead';

export type TimelineItem = {
	id: string;
	start: number;
	end: number;
	included: boolean;
	kind: 'segment' | 'gap';
	segmentId?: string;
	edited: boolean;
};

export type DragDirection = 'forward' | 'backward';
