<script lang="ts">
	import { enhance } from '$app/forms';
	import { invalidateAll } from '$app/navigation';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as studio from '$lib/components/studio';
	import { formatTime } from '$lib/components/studio/helpers';
	import { activeWordId, wordsFromSegments } from '$lib/components/studio/transcript-panel/helpers';
	import type { TimelineItem } from '$lib/components/studio/timeline/types';
	import { itemsFromSegments } from '$lib/components/studio/timeline/helpers';
	import {
		nextPlaybackTick,
		playbackStart,
		stopTime
	} from '$lib/components/studio/video-preview/helpers';
	import { isIndeterminate, isProcessing, isReady } from '$lib/concepts/core/service';
	import * as videoSegments from '$lib/concepts/video/segments';
	import * as transcription from '$lib/concepts/video/transcription';
	import { tick } from 'svelte';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type { PageServerData } from './$types';

	let { data }: { data: PageServerData } = $props();
	let player = $state<HTMLVideoElement>();
	let playingPreview = $state(false);
	let activePreviewIndex = $state(-1);
	let previewFrame = $state<number>();
	let currentSourceTime = $state(0);
	let localSegments = $state<videoSegments.Segment[]>([]);
	let segmentVersion = $state(-1);
	let loadedSegmentVersion = $state(-1);
	let segmentSaveError = $state('');
	let savingSegmentId = $state('');
	let editedSegmentIds = $state<string[]>([]);
	let transcribeElapsedSeconds = $state(0);
	let saveSegmentsForm = $state<HTMLFormElement>();
	let pendingBaseVersion = $state(0);
	let pendingSegments = $state<videoSegments.Segment[]>([]);
	let previousSegments = $state<videoSegments.Segment[]>([]);
	let previousVersion = $state(0);
	let previousEditedSegmentIds = $state<string[]>([]);

	const transcript = $derived(transcription.textForDisplay(data.job.analysis));
	const duration = $derived(
		data.job.analysis?.inputDurationSeconds ?? videoSegments.duration(localSegments)
	);
	const ready = $derived(isReady(data.job));
	const processing = $derived(isProcessing(data.job));
	const indeterminate = $derived(isIndeterminate(data.job));
	const timelineItems = $derived(itemsFromSegments(localSegments, duration, editedSegmentIds));
	const acceptedCount = $derived(videoSegments.includedCount(localSegments));
	const previewRanges = $derived(videoSegments.previewRanges(localSegments));
	const trimmedDuration = $derived(videoSegments.previewDuration(previewRanges));
	const removedDuration = $derived(Math.max(0, duration - trimmedDuration));
	const transcriptWords = $derived(wordsFromSegments(localSegments));
	const currentActiveWordId = $derived(activeWordId(transcriptWords, currentSourceTime));

	$effect.pre(() => {
		const version = data.segments?.version ?? 0;
		if (version !== loadedSegmentVersion) {
			localSegments = data.segments?.segments ?? [];
			segmentVersion = version;
			loadedSegmentVersion = version;
		}
	});

	$effect(() => {
		if (!processing) {
			return;
		}

		const interval = window.setInterval(() => {
			void invalidateAll();
		}, 2500);

		return () => {
			window.clearInterval(interval);
		};
	});

	$effect(() => {
		if (!ready || data.waveform || data.media.waveformStatus !== 'generating') {
			return;
		}

		const interval = window.setInterval(() => {
			void invalidateAll();
		}, 2500);

		return () => {
			window.clearInterval(interval);
		};
	});

	$effect(() => {
		if (!indeterminate) {
			transcribeElapsedSeconds = 0;
			return;
		}

		const startedAt = Date.now();
		transcribeElapsedSeconds = 0;

		const interval = window.setInterval(() => {
			transcribeElapsedSeconds = Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
		}, 1000);

		return () => {
			window.clearInterval(interval);
		};
	});

	const seekToSourceTime = (time: number) => {
		if (!player) {
			return;
		}

		pausePreview();
		player.currentTime = time;
		currentSourceTime = time;
	};

	const enhanceSaveSegments: SubmitFunction = () => {
		return async ({ result }) => {
			if (result.type === 'success') {
				const data = result.data as { version?: number } | undefined;
				segmentVersion = data?.version ?? segmentVersion;
			} else {
				localSegments = previousSegments;
				segmentVersion = previousVersion;
				editedSegmentIds = previousEditedSegmentIds;
				segmentSaveError =
					result.type === 'failure' && result.data && 'message' in result.data
						? String(result.data.message)
						: 'Could not save segment changes.';
			}

			savingSegmentId = '';
		};
	};

	const saveSegments = (
		nextSegments: videoSegments.Segment[],
		savingId = '',
		markEditedId = savingId
	) => {
		previousSegments = localSegments;
		previousVersion = segmentVersion;
		previousEditedSegmentIds = editedSegmentIds;
		segmentSaveError = '';
		savingSegmentId = savingId;
		pausePreview();
		pendingBaseVersion = segmentVersion;
		pendingSegments = nextSegments;
		localSegments = nextSegments;
		if (markEditedId && !editedSegmentIds.includes(markEditedId)) {
			editedSegmentIds = [...editedSegmentIds, markEditedId];
		}

		void tick().then(() => saveSegmentsForm?.requestSubmit());
	};

	const toggleSegmentIncluded = (segmentId: string, included: boolean) => {
		saveSegments(videoSegments.toggleIncluded(localSegments, segmentId, included), segmentId);
	};

	const addGapToCut = (item: TimelineItem) => {
		if (item.kind !== 'gap') {
			return;
		}

		const manualSegment = videoSegments.createManual({ start: item.start, end: item.end });
		saveSegments(videoSegments.insert(localSegments, manualSegment), manualSegment.id);
	};

	const togglePreview = () => {
		if (playingPreview) {
			pausePreview();
			return;
		}

		void playPreview();
	};

	const playPreview = async () => {
		if (!player || previewRanges.length === 0) {
			return;
		}

		const start = playbackStart(previewRanges, player.currentTime);
		activePreviewIndex = start.index;
		player.currentTime = start.time;
		currentSourceTime = start.time;

		playingPreview = true;
		await player.play();
		previewFrame = window.requestAnimationFrame(tickPreview);
	};

	const pausePreview = () => {
		if (previewFrame) {
			window.cancelAnimationFrame(previewFrame);
			previewFrame = undefined;
		}

		player?.pause();
		playingPreview = false;
	};

	const stopPreview = () => {
		pausePreview();
		activePreviewIndex = -1;
		const time = stopTime(previewRanges);
		if (player && time !== undefined) {
			player.currentTime = time;
			currentSourceTime = time;
		}
	};

	const tickPreview = () => {
		if (!player || !playingPreview) {
			return;
		}

		currentSourceTime = player.currentTime;
		const tick = nextPlaybackTick(previewRanges, activePreviewIndex, player.currentTime);
		if (tick.action === 'stop') {
			stopPreview();
			return;
		}

		if (tick.action === 'finish') {
			player.currentTime = tick.time;
			currentSourceTime = tick.time;
			stopPreview();
			return;
		}

		if (tick.action === 'next') {
			activePreviewIndex = tick.index;
			player.currentTime = tick.time;
			currentSourceTime = tick.time;
		}

		previewFrame = window.requestAnimationFrame(tickPreview);
	};

	function setPlayerSourceTime(time: number) {
		pausePreview();
		if (player) {
			player.currentTime = time;
		}
		currentSourceTime = time;
	}
</script>

<svelte:head>
	<title>Studio | Trimia</title>
</svelte:head>

<form
	bind:this={saveSegmentsForm}
	method="POST"
	action="?/saveSegments"
	use:enhance={enhanceSaveSegments}
	class="hidden"
>
	<input type="hidden" name="baseVersion" value={pendingBaseVersion} />
	<input type="hidden" name="segments" value={JSON.stringify(pendingSegments)} />
</form>

<main class="h-dvh overflow-hidden bg-background px-4 py-3 sm:px-6 lg:px-8">
	<section class="mx-auto flex h-full w-full max-w-7xl flex-col gap-3">
		<header class="flex items-center justify-between gap-4">
			<div>
				<h1 class="text-2xl font-semibold tracking-tight">Trimia Studio</h1>
				<p class="text-xs text-muted-foreground">Job {data.job.jobId}</p>
			</div>
		</header>

		{#if processing}
			<studio.ProcessingStatus
				phase={data.job.phase}
				progress={data.job.progress}
				{indeterminate}
				elapsedSeconds={transcribeElapsedSeconds}
			/>
		{/if}

		{#if data.job.error}
			<Card.Root class="border-destructive/40 bg-destructive/5">
				<Card.Header>
					<Card.Title>Analysis failed</Card.Title>
					<Card.Description>{data.job.error}</Card.Description>
				</Card.Header>
			</Card.Root>
		{/if}

		{#if segmentSaveError}
			<Card.Root class="shrink-0 border-destructive/40 bg-destructive/5">
				<Card.Content class="py-3 text-sm text-destructive">{segmentSaveError}</Card.Content>
			</Card.Root>
		{/if}

		<div class="grid min-h-0 flex-1 gap-3 lg:grid-cols-[minmax(0,1.7fr)_minmax(21rem,0.85fr)]">
			<studio.VideoPreview
				bind:player
				sourceUrl={data.sourceUrl}
				{ready}
				disabled={previewRanges.length === 0}
				playing={playingPreview}
				{currentSourceTime}
				{duration}
				onTimeUpdate={(time) => (currentSourceTime = time)}
				onToggle={togglePreview}
				onStop={stopPreview}
			/>

			<studio.TranscriptPanel
				words={transcriptWords}
				activeWordId={currentActiveWordId}
				fallbackTranscript={transcript}
				onSeek={seekToSourceTime}
			/>
		</div>

		<studio.Timeline
			items={timelineItems}
			waveform={data.waveform}
			{previewRanges}
			{duration}
			{trimmedDuration}
			{removedDuration}
			{acceptedCount}
			{currentSourceTime}
			{playingPreview}
			{savingSegmentId}
			onSeek={seekToSourceTime}
			onAddGap={addGapToCut}
			onToggleSegment={toggleSegmentIncluded}
			onDragTime={setPlayerSourceTime}
			onRefresh={() => void invalidateAll()}
		/>
	</section>
</main>
