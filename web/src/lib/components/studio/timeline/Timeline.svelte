<script lang="ts">
	import type * as segments from '$lib/concepts/video/segments';
	import type * as core from '$lib/concepts/core';
	import * as Card from '$lib/components/ui/card/index.js';
	import { formatTime } from '../helpers';
	import EmptyTimeline from './EmptyTimeline.svelte';
	import TimelineBar from './TimelineBar.svelte';
	import TimelineWaveform from './TimelineWaveform.svelte';
	import TimelineZoomControl from './TimelineZoomControl.svelte';
	import { contentWidth, itemWidth, playheadLeft, resolveAcceptedDragTime } from './helpers';
	import type { TimelineItem, TimelineZoom } from './types';

	const pixelsPerSecond = 48;

	let {
		items,
		waveform,
		previewRanges,
		duration,
		trimmedDuration,
		removedDuration,
		acceptedCount,
		currentSourceTime,
		playingPreview,
		savingSegmentId,
		onSeek,
		onAddGap,
		onToggleSegment,
		onDragTime,
		onRefresh
	}: {
		items: TimelineItem[];
		waveform?: core.Waveform;
		previewRanges: segments.PreviewRange[];
		duration: number;
		trimmedDuration: number;
		removedDuration: number;
		acceptedCount: number;
		currentSourceTime: number;
		playingPreview: boolean;
		savingSegmentId: string;
		onSeek: (time: number) => void;
		onAddGap: (item: TimelineItem) => void;
		onToggleSegment: (segmentId: string, included: boolean) => void;
		onDragTime: (time: number) => void;
		onRefresh: () => void;
	} = $props();

	let zoom = $state<TimelineZoom>('playhead');
	let timelineElement = $state<HTMLDivElement>();
	let scrollElement = $state<HTMLDivElement>();
	let draggingPlayhead = $state(false);
	let lastDragSourceTime = $state(0);

	const timelineContentWidth = $derived(contentWidth(duration, zoom, pixelsPerSecond));
	const playheadPosition = $derived(
		playheadLeft(currentSourceTime, duration, zoom, pixelsPerSecond)
	);

	$effect(() => {
		if (zoom !== 'playhead' || !scrollElement || duration <= 0) {
			return;
		}

		if (playingPreview || draggingPlayhead) {
			centerOnTime(currentSourceTime);
		}
	});

	function setZoom(nextZoom: TimelineZoom) {
		zoom = nextZoom;
		if (nextZoom === 'playhead') {
			setTimeout(() => centerOnTime(currentSourceTime));
		}
	}

	function startDrag(event: PointerEvent) {
		if (!timelineElement || previewRanges.length === 0) {
			return;
		}

		draggingPlayhead = true;
		lastDragSourceTime = currentSourceTime;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		drag(event);
	}

	function drag(event: PointerEvent) {
		if (!draggingPlayhead || !timelineElement || duration <= 0) {
			return;
		}

		const rect = timelineElement.getBoundingClientRect();
		const percent = Math.min(Math.max((event.clientX - rect.left) / rect.width, 0), 1);
		const rawTime = percent * duration;
		const resolvedTime = resolveAcceptedDragTime(
			previewRanges,
			rawTime,
			rawTime >= lastDragSourceTime ? 'forward' : 'backward'
		);

		onDragTime(resolvedTime);
		lastDragSourceTime = rawTime;
		centerOnTime(resolvedTime);
	}

	function endDrag(event: PointerEvent) {
		if (!draggingPlayhead) {
			return;
		}

		draggingPlayhead = false;
		(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId);
	}

	function centerOnTime(time: number) {
		if (!scrollElement || zoom !== 'playhead') {
			return;
		}

		const targetLeft = time * pixelsPerSecond - scrollElement.clientWidth / 2;
		scrollElement.scrollLeft = Math.max(0, targetLeft);
	}
</script>

<Card.Root class="shrink-0 shadow-sm">
	<Card.Header class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
		<Card.Title>Timeline</Card.Title>
		{#if items.length > 0}
			<div class="flex flex-wrap items-center gap-3 text-sm">
				<div class="flex flex-wrap gap-3 text-muted-foreground">
					<span>{formatTime(removedDuration)} trimmed</span>
					<span>{formatTime(trimmedDuration)} final</span>
				</div>
				<TimelineZoomControl {zoom} onChange={setZoom} />
			</div>
		{/if}
	</Card.Header>
	<Card.Content class="space-y-2 pb-4">
		{#if items.length > 0}
			<div class="space-y-2">
				<div
					bind:this={scrollElement}
					class={[
						zoom === 'playhead' ? 'overflow-x-auto pb-1' : 'overflow-x-hidden',
						'w-full'
					].join(' ')}
				>
					<div
						bind:this={timelineElement}
						class="relative pt-3"
						style={`width: ${timelineContentWidth}`}
					>
						<div
							class="absolute top-0 bottom-0 z-10 -translate-x-1/2"
							style={`left: ${playheadPosition}`}
						>
							<button
								type="button"
								class="mx-auto block h-0 w-0 cursor-ew-resize border-x-[8px] border-t-[12px] border-x-transparent border-t-red-500 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
								onpointerdown={startDrag}
								onpointermove={drag}
								onpointerup={endDrag}
								onpointercancel={endDrag}
								aria-label="Drag preview playhead"
							></button>
							<div
								class="pointer-events-none mx-auto h-[4rem] w-0.5 bg-red-500 shadow-[0_0_0_1px_rgba(255,255,255,0.55)]"
							></div>
						</div>
						<div class="relative flex h-16 overflow-hidden rounded-md border bg-muted/25">
							<TimelineWaveform {waveform} {duration} />
							{#each items as item, index (item.id)}
								<TimelineBar
									{item}
									{index}
									width={itemWidth(item, duration, zoom, pixelsPerSecond)}
									{savingSegmentId}
									{onSeek}
									{onAddGap}
									{onToggleSegment}
								/>
							{/each}
						</div>
					</div>
				</div>
				<div class="flex justify-between text-xs text-muted-foreground">
					<span>0:00</span>
					<span>{formatTime(duration)}</span>
				</div>
			</div>
		{:else}
			<EmptyTimeline {onRefresh} />
		{/if}
	</Card.Content>
</Card.Root>
