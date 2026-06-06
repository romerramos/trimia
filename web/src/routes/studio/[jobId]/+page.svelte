<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import type { PageServerData } from './$types';

	let { data }: { data: PageServerData } = $props();
	let player = $state<HTMLVideoElement>();
	let playingPreview = $state(false);
	let activePreviewIndex = $state(-1);
	let previewFrame = $state<number>();
	let currentSourceTime = $state(0);
	let timelineElement = $state<HTMLDivElement>();
	let timelineScrollElement = $state<HTMLDivElement>();
	let draggingPlayhead = $state(false);
	let lastDragSourceTime = $state(0);
	let timelineZoom = $state<'fit' | 'playhead'>('fit');
	type TrimiaSegment = NonNullable<PageServerData['segments']>['segments'][number];
	let localSegments = $state<TrimiaSegment[]>([]);
	let segmentVersion = $state(-1);
	let loadedSegmentVersion = $state(-1);
	let segmentSaveError = $state('');
	let savingSegmentId = $state('');
	let editedSegmentIds = $state<string[]>([]);

	const previewPreRoll = 0.03;
	const previewPostRoll = 0.06;
	const previewMergeGap = 0.12;
	const zoomPixelsPerSecond = 48;

	const segments = $derived(localSegments);
	const transcript = $derived(data.job.analysis?.cleanTranscript || data.job.analysis?.originalTranscript || 'Transcript will appear when analysis completes.');
	const duration = $derived(data.job.analysis?.inputDurationSeconds ?? Math.max(...segments.map((segment) => segment.end), 0));
	const ready = $derived(data.job.status === 'awaiting_confirmation' || data.job.status === 'completed' || data.job.status === 'rendering');
	const processing = $derived(data.job.status === 'queued' || data.job.status === 'running');
	type TimelineItem = {
		id: string;
		start: number;
		end: number;
		included: boolean;
		kind: 'segment' | 'gap';
		segmentId?: string;
		edited: boolean;
	};
	type TranscriptWord = {
		id: string;
		text: string;
		start: number;
		end: number;
		included: boolean;
	};

	const timelineItems = $derived.by(() => {
		if (duration <= 0 || segments.length === 0) {
			return [] as TimelineItem[];
		}

		const items: TimelineItem[] = [];
		let cursor = 0;

		for (const segment of segments.toSorted((a, b) => a.start - b.start)) {
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
	});
	const acceptedCount = $derived(segments.filter((segment) => segment.included).length);
	const removedCount = $derived(timelineItems.filter((item) => !item.included).length);
	const previewSegments = $derived.by(() => {
		const included = segments
			.filter((segment) => segment.included)
			.map((segment) => ({
				id: segment.id,
				start: Math.max(0, segment.start - previewPreRoll),
				end: segment.end + previewPostRoll
			}))
			.toSorted((a, b) => a.start - b.start);

		const merged: { id: string; start: number; end: number }[] = [];
		for (const segment of included) {
			const last = merged.at(-1);
			if (last && segment.start <= last.end + previewMergeGap) {
				last.end = Math.max(last.end, segment.end);
				last.id = `${last.id}-${segment.id}`;
				continue;
			}

			merged.push({ ...segment });
		}

		return merged;
	});
	const previewDuration = $derived(previewSegments.reduce((total, segment) => total + segment.end - segment.start, 0));
	const playheadPercent = $derived(duration > 0 ? Math.min(Math.max((currentSourceTime / duration) * 100, 0), 100) : 0);
	const playheadLeft = $derived(timelineZoom === 'fit' ? `${playheadPercent}%` : `${currentSourceTime * zoomPixelsPerSecond}px`);
	const timelineContentWidth = $derived(timelineZoom === 'fit' ? '100%' : `${Math.max(duration * zoomPixelsPerSecond, 960)}px`);
	const currentPreviewTime = $derived(previewTimeFromSourceTime(currentSourceTime));
	const transcriptWords = $derived.by(() => {
		return segments.flatMap((segment) => {
			return (segment.words ?? []).map((word, index) => ({
				id: `${segment.id}-${index}-${word.start}`,
				text: word.punctuatedWord || word.word,
				start: word.start,
				end: word.end,
				included: segment.included
			})) satisfies TranscriptWord[];
		});
	});
	const activeWordId = $derived(transcriptWords.find((word) => currentSourceTime >= word.start && currentSourceTime < word.end)?.id ?? '');

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
		if (timelineZoom !== 'playhead' || !timelineScrollElement || duration <= 0) {
			return;
		}

		if (playingPreview || draggingPlayhead) {
			centerTimelineOnTime(currentSourceTime);
		}
	});

	const seekToSegment = (start: number) => {
		seekToSourceTime(start);
	};

	const seekToSourceTime = (time: number) => {
		if (!player) {
			return;
		}

		pausePreview();
		player.currentTime = time;
		currentSourceTime = time;
	};

	const saveSegments = async (nextSegments: TrimiaSegment[], savingId = '', markEditedId = savingId) => {
		const previousSegments = localSegments;
		const previousVersion = segmentVersion;
		const previousEditedSegmentIds = editedSegmentIds;
		segmentSaveError = '';
		savingSegmentId = savingId;
		pausePreview();
		localSegments = nextSegments;
		if (markEditedId && !editedSegmentIds.includes(markEditedId)) {
			editedSegmentIds = [...editedSegmentIds, markEditedId];
		}

		try {
			const response = await fetch(data.segmentsUrl, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					baseVersion: previousVersion,
					segments: nextSegments
				})
			});

			if (!response.ok) {
				throw new Error((await response.text()) || `Trimia returned ${response.status}`);
			}

			const result = (await response.json()) as { version: number };
			segmentVersion = result.version;
		} catch (error) {
			localSegments = previousSegments;
			segmentVersion = previousVersion;
			editedSegmentIds = previousEditedSegmentIds;
			segmentSaveError = error instanceof Error ? error.message : 'Could not save segment changes.';
		} finally {
			savingSegmentId = '';
		}
	};

	const toggleSegmentIncluded = async (segmentId: string, included: boolean) => {
		await saveSegments(
			localSegments.map((segment) => (segment.id === segmentId ? { ...segment, included } : segment)),
			segmentId
		);
	};

	const addGapToCut = async (item: TimelineItem) => {
		if (item.kind !== 'gap') {
			return;
		}

		const segmentId = `manual_${Date.now().toString(36)}_${Math.round(item.start * 1000).toString(36)}`;
		const manualSegment: TrimiaSegment = {
			id: segmentId,
			start: item.start,
			end: item.end,
			text: '',
			source: 'manual',
			included: true,
			words: []
		};

		await saveSegments([...localSegments, manualSegment].toSorted((a, b) => a.start - b.start), segmentId);
	};

	const contextMenuLabel = (item: TimelineItem) => {
		if (item.kind === 'gap') {
			return 'Add to cut';
		}

		return item.included ? 'Remove from cut' : 'Add to cut';
	};

	const timelineItemClass = (item: TimelineItem) => {
		return [
			item.included ? 'bg-foreground hover:bg-foreground/90' : 'bg-destructive/25 hover:bg-destructive/35',
			item.edited ? 'opacity-65 ring-1 ring-inset ring-primary/50' : '',
			savingSegmentId === item.segmentId ? 'opacity-50' : '',
			'h-full w-full transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
		].join(' ');
	};

	const togglePreview = () => {
		if (playingPreview) {
			pausePreview();
			return;
		}

		void playPreview();
	};

	const playPreview = async () => {
		if (!player || previewSegments.length === 0) {
			return;
		}

		const index = findPreviewIndex(player.currentTime);
		const segment = previewSegments[index];
		activePreviewIndex = index;
		if (player.currentTime < segment.start || player.currentTime >= segment.end) {
			player.currentTime = segment.start;
			currentSourceTime = segment.start;
		}

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
		const firstSegment = previewSegments[0];
		if (player && firstSegment) {
			player.currentTime = firstSegment.start;
			currentSourceTime = firstSegment.start;
		}
	};

	const tickPreview = () => {
		if (!player || !playingPreview) {
			return;
		}

		currentSourceTime = player.currentTime;
		const segment = previewSegments[activePreviewIndex];
		if (!segment) {
			stopPreview();
			return;
		}

		if (player.currentTime >= segment.end) {
			const next = previewSegments[activePreviewIndex + 1];
			if (!next) {
				player.currentTime = segment.end;
				currentSourceTime = segment.end;
				stopPreview();
				return;
			}

			activePreviewIndex += 1;
			player.currentTime = next.start;
			currentSourceTime = next.start;
		}

		previewFrame = window.requestAnimationFrame(tickPreview);
	};

	const findPreviewIndex = (time: number) => {
		const containing = previewSegments.findIndex((segment) => time >= segment.start && time < segment.end);
		if (containing !== -1) {
			return containing;
		}

		const next = previewSegments.findIndex((segment) => segment.start > time);
		return next === -1 ? 0 : next;
	};

	const startPlayheadDrag = (event: PointerEvent) => {
		if (!timelineElement || previewSegments.length === 0) {
			return;
		}

		pausePreview();
		draggingPlayhead = true;
		lastDragSourceTime = currentSourceTime;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		dragPlayhead(event);
	};

	const dragPlayhead = (event: PointerEvent) => {
		if (!draggingPlayhead || !timelineElement || !player || duration <= 0) {
			return;
		}

		const rect = timelineElement.getBoundingClientRect();
		const percent = Math.min(Math.max((event.clientX - rect.left) / rect.width, 0), 1);
		const rawTime = percent * duration;
		const resolvedTime = resolveAcceptedDragTime(rawTime, rawTime >= lastDragSourceTime ? 'forward' : 'backward');

		player.currentTime = resolvedTime;
		currentSourceTime = resolvedTime;
		lastDragSourceTime = rawTime;
		centerTimelineOnTime(resolvedTime);
	};

	const endPlayheadDrag = (event: PointerEvent) => {
		if (!draggingPlayhead) {
			return;
		}

		draggingPlayhead = false;
		(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId);
	};

	const resolveAcceptedDragTime = (time: number, direction: 'forward' | 'backward') => {
		const containing = previewSegments.find((segment) => time >= segment.start && time <= segment.end);
		if (containing) {
			return time;
		}

		if (direction === 'forward') {
			const next = previewSegments.find((segment) => segment.start > time);
			return next?.start ?? previewSegments.at(-1)?.end ?? 0;
		}

		const previous = previewSegments.toReversed().find((segment) => segment.end < time);
		return previous?.end ?? previewSegments[0]?.start ?? 0;
	};

	function previewTimeFromSourceTime(sourceTime: number) {
		let elapsed = 0;

		for (const segment of previewSegments) {
			if (sourceTime < segment.start) {
				return elapsed;
			}
			if (sourceTime <= segment.end) {
				return elapsed + sourceTime - segment.start;
			}
			elapsed += segment.end - segment.start;
		}

		return elapsed;
	}

	const formatTime = (seconds: number) => {
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = Math.floor(seconds % 60);
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	};

	const itemWidth = (start: number, end: number) => {
		if (timelineZoom === 'playhead') {
			return `${Math.max((end - start) * zoomPixelsPerSecond, 10)}px`;
		}

		if (duration <= 0) {
			return '0%';
		}

		const percent = ((end - start) / duration) * 100;
		return `${percent}%`;
	};

	const centerTimelineOnTime = (time: number) => {
		if (!timelineScrollElement || timelineZoom !== 'playhead') {
			return;
		}

		const targetLeft = time * zoomPixelsPerSecond - timelineScrollElement.clientWidth / 2;
		timelineScrollElement.scrollLeft = Math.max(0, targetLeft);
	};

</script>

<svelte:head>
	<title>Studio | Trimia</title>
</svelte:head>

<main class="bg-background h-dvh overflow-hidden px-4 py-3 sm:px-6 lg:px-8">
	<section class="mx-auto flex h-full w-full max-w-7xl flex-col gap-3">
		<header class="flex items-center justify-between gap-4">
			<div>
				<h1 class="text-2xl font-semibold tracking-tight">Trimia Studio</h1>
				<p class="text-muted-foreground text-xs">Job {data.job.jobId}</p>
			</div>

			{#if processing}
				<div class="text-muted-foreground flex items-center gap-2 text-sm">
					<span class="bg-primary h-2 w-2 animate-pulse rounded-full"></span>
					<span class="capitalize">{data.job.phase.replaceAll('_', ' ')}</span>
					<span>{Math.round(data.job.progress)}%</span>
				</div>
			{/if}
		</header>

		{#if processing}
			<Card.Root class="shadow-sm">
				<Card.Content class="pt-6">
					<div class="space-y-3">
						<div class="flex justify-between gap-4 text-sm">
							<span class="font-medium capitalize">{data.job.phase.replaceAll('_', ' ')}</span>
							<span class="text-muted-foreground">{Math.round(data.job.progress)}%</span>
						</div>
						<div class="bg-muted h-2 overflow-hidden rounded-full">
							<div class="bg-primary h-full rounded-full transition-all duration-500" style={`width: ${Math.max(data.job.progress, 4)}%`}></div>
						</div>
					</div>
				</Card.Content>
			</Card.Root>
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
			<Card.Root class="border-destructive/40 bg-destructive/5 shrink-0">
				<Card.Content class="py-3 text-sm text-destructive">{segmentSaveError}</Card.Content>
			</Card.Root>
		{/if}

		<div class="grid min-h-0 flex-1 gap-3 lg:grid-cols-[minmax(0,1.7fr)_minmax(21rem,0.85fr)]">
			<div class="flex min-h-0 items-stretch">
				<div class="flex h-full max-h-full w-full flex-col overflow-hidden rounded-xl bg-black shadow-sm">
					<video bind:this={player} class="min-h-0 flex-1 bg-black object-contain" src={data.sourceUrl} preload="metadata" ontimeupdate={() => (currentSourceTime = player?.currentTime ?? 0)} onended={stopPreview}>
						<track kind="captions" />
					</video>

					<div class="flex shrink-0 items-center justify-between gap-4 border-t border-white/10 bg-zinc-950 px-4 py-3 text-white">
						<div class="flex items-center gap-2">
							<button
								type="button"
								class="flex h-10 w-10 items-center justify-center rounded-full bg-white text-sm font-semibold text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-40"
								disabled={!ready || previewSegments.length === 0}
								onclick={togglePreview}
								aria-label={playingPreview ? 'Pause preview' : 'Play preview'}
							>
								{playingPreview ? 'Ⅱ' : '▶'}
							</button>
							<button
								type="button"
								class="flex h-10 w-10 items-center justify-center rounded-full border border-white/20 text-sm font-semibold text-white transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-40"
								disabled={!ready || previewSegments.length === 0}
								onclick={stopPreview}
								aria-label="Stop preview"
							>
								■
							</button>
						</div>

						<div class="font-mono text-sm tabular-nums text-zinc-300">
							{formatTime(currentPreviewTime)} / {formatTime(previewDuration)}
						</div>
					</div>
				</div>
			</div>

			<Card.Root class="min-h-0 overflow-hidden shadow-sm">
				<Card.Header>
					<Card.Title>Transcript</Card.Title>
				</Card.Header>
				<Card.Content class="min-h-0 pb-4">
					{#if transcriptWords.length > 0}
						<div class="max-h-[calc(100dvh-16rem)] overflow-y-auto pr-1 text-sm leading-8">
							{#each transcriptWords as word (word.id)}
								<button
									type="button"
									class={[
										word.id === activeWordId ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
										'inline rounded px-1 py-0.5 text-left transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
									].join(' ')}
									onclick={() => seekToSourceTime(word.start)}
									aria-label={`Jump to ${word.text} at ${formatTime(word.start)}`}
								>
									{word.text}
								</button>{' '}
							{/each}
						</div>
					{:else}
						<p class="text-muted-foreground max-h-[36rem] overflow-y-auto text-sm leading-7 whitespace-pre-wrap">{transcript}</p>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>

		<Card.Root class="shrink-0 shadow-sm">
			<Card.Header class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
				<Card.Title>Timeline</Card.Title>
				{#if timelineItems.length > 0}
					<div class="flex flex-wrap items-center gap-3 text-sm">
						<div class="text-muted-foreground flex flex-wrap gap-3">
							<span>{acceptedCount} accepted</span>
							<span>{removedCount} removed gaps</span>
							<span>{formatTime(duration)} source</span>
						</div>
						<div class="bg-muted flex rounded-lg p-1">
							<button
								type="button"
								class={[timelineZoom === 'fit' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground', 'rounded-md px-2 py-1 text-xs font-medium transition'].join(' ')}
								onclick={() => (timelineZoom = 'fit')}
							>
								Full width
							</button>
							<button
								type="button"
								class={[timelineZoom === 'playhead' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground', 'rounded-md px-2 py-1 text-xs font-medium transition'].join(' ')}
								onclick={() => {
									timelineZoom = 'playhead';
									setTimeout(() => centerTimelineOnTime(currentSourceTime));
								}}
							>
								Zoom on playhead
							</button>
						</div>
					</div>
				{/if}
			</Card.Header>
			<Card.Content class="space-y-2 pb-4">
				{#if timelineItems.length > 0}
					<div class="space-y-2">
						<div bind:this={timelineScrollElement} class={[timelineZoom === 'playhead' ? 'overflow-x-auto pb-1' : 'overflow-x-hidden', 'w-full'].join(' ')}>
						<div bind:this={timelineElement} class="relative pt-3" style={`width: ${timelineContentWidth}`}>
							<div class="absolute top-0 bottom-0 z-10 -translate-x-1/2" style={`left: ${playheadLeft}`}>
								<button
									type="button"
									class="mx-auto block h-0 w-0 cursor-ew-resize border-x-[8px] border-t-[12px] border-x-transparent border-t-red-500 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
									onpointerdown={startPlayheadDrag}
									onpointermove={dragPlayhead}
									onpointerup={endPlayheadDrag}
									onpointercancel={endPlayheadDrag}
									aria-label="Drag preview playhead"
								></button>
								<div class="pointer-events-none mx-auto h-[4rem] w-0.5 bg-red-500 shadow-[0_0_0_1px_rgba(255,255,255,0.55)]"></div>
							</div>
							<div class="flex h-16 overflow-hidden rounded-md">
								{#each timelineItems as item, index (item.id)}
									<ContextMenu.Root>
										<ContextMenu.Trigger style={`width: ${itemWidth(item.start, item.end)}`} class="h-full min-w-1">
											<button
												type="button"
												class={timelineItemClass(item)}
												title={`${item.included ? 'Accepted' : 'Removed'} ${formatTime(item.start)} to ${formatTime(item.end)}`}
												onclick={() => seekToSegment(item.start)}
												aria-label={`Jump to ${item.included ? 'accepted segment' : 'removed time'} ${index + 1} at ${formatTime(item.start)}`}
											></button>
										</ContextMenu.Trigger>
										<ContextMenu.Content class="z-50 min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md">
											<ContextMenu.Item
												class="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50"
												disabled={!!item.segmentId && savingSegmentId === item.segmentId}
												onSelect={() => {
													if (item.kind === 'gap') {
														void addGapToCut(item);
													} else if (item.segmentId) {
														void toggleSegmentIncluded(item.segmentId, !item.included);
													}
												}}
											>
												{contextMenuLabel(item)}
											</ContextMenu.Item>
										</ContextMenu.Content>
									</ContextMenu.Root>
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
					<div class="bg-muted/60 rounded-xl border border-dashed p-8 text-center">
						<p class="font-medium">Timeline is waiting for analysis.</p>
						<p class="text-muted-foreground mt-2 text-sm">Trimia will add segments here as soon as the transcript analysis completes.</p>
						<Button class="mt-4" variant="outline" onclick={() => invalidateAll()}>Refresh</Button>
					</div>
				{/if}
			</Card.Content>
		</Card.Root>
	</section>
</main>
