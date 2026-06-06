<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import type { PageServerData } from './$types';

	let { data }: { data: PageServerData } = $props();
	let player = $state<HTMLVideoElement>();
	let playingPreview = $state(false);
	let activePreviewIndex = $state(-1);
	let previewFrame = $state<number>();
	let currentSourceTime = $state(0);
	let timelineElement = $state<HTMLDivElement>();
	let draggingPlayhead = $state(false);
	let lastDragSourceTime = $state(0);

	const previewPreRoll = 0.03;
	const previewPostRoll = 0.06;
	const previewMergeGap = 0.12;

	const segments = $derived(data.segments?.segments ?? []);
	const transcript = $derived(data.job.analysis?.cleanTranscript || data.job.analysis?.originalTranscript || 'Transcript will appear when analysis completes.');
	const duration = $derived(data.job.analysis?.inputDurationSeconds ?? Math.max(...segments.map((segment) => segment.end), 0));
	const ready = $derived(data.job.status === 'awaiting_confirmation' || data.job.status === 'completed' || data.job.status === 'rendering');
	const processing = $derived(data.job.status === 'queued' || data.job.status === 'running');
	type TimelineItem = {
		id: string;
		start: number;
		end: number;
		included: boolean;
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
					included: false
				});
			}

			items.push({
				id: segment.id,
				start: segment.start,
				end: segment.end,
				included: segment.included
			});
			cursor = Math.max(cursor, segment.end);
		}

		if (cursor < duration) {
			items.push({
				id: `gap-${cursor}-${duration}`,
				start: cursor,
				end: duration,
				included: false
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
		if (duration <= 0) {
			return '0%';
		}

		const percent = ((end - start) / duration) * 100;
		return `${percent}%`;
	};

</script>

<svelte:head>
	<title>Studio | Trimia</title>
</svelte:head>

<main class="bg-background min-h-screen px-4 py-6 sm:px-6 lg:px-8">
	<section class="mx-auto flex w-full max-w-7xl flex-col gap-6">
		<header class="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
			<div class="space-y-2">
				<p class="text-muted-foreground text-sm font-medium">Studio · Job {data.job.jobId}</p>
				<h1 class="text-3xl font-semibold tracking-tight text-balance">Review your cut</h1>
				<p class="text-muted-foreground max-w-2xl text-pretty">
					Use the timeline to jump through the accepted and removed segments. This first studio pass keeps editing simple while the full segment controls take shape.
				</p>
			</div>

			<div class="flex items-center gap-3 rounded-full border px-4 py-2 text-sm">
				<span class={[processing ? 'animate-pulse bg-primary' : 'bg-primary', 'h-2 w-2 rounded-full'].join(' ')}></span>
				<span class="font-medium capitalize">{data.job.status.replaceAll('_', ' ')}</span>
				<span class="text-muted-foreground">{Math.round(data.job.progress)}%</span>
			</div>
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

		<div class="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(22rem,0.9fr)]">
			<div class="space-y-6">
				<Card.Root class="overflow-hidden shadow-sm">
					<Card.Header>
						<Card.Title>Preview</Card.Title>
						<Card.Description>Play Preview jumps across accepted segments to approximate the final render.</Card.Description>
					</Card.Header>
					<Card.Content>
						<div class="overflow-hidden rounded-xl border bg-black shadow-sm">
							<video bind:this={player} class="aspect-video w-full bg-black object-contain" src={data.sourceUrl} preload="metadata" ontimeupdate={() => (currentSourceTime = player?.currentTime ?? 0)} onended={stopPreview}>
								<track kind="captions" />
							</video>

							<div class="flex items-center justify-between gap-4 border-t border-white/10 bg-zinc-950 px-4 py-3 text-white">
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
					</Card.Content>
				</Card.Root>

				<Card.Root class="shadow-sm">
					<Card.Header>
						<Card.Title>Timeline</Card.Title>
					<Card.Description>{ready ? 'The bar maps the source video from start to finish. Dark blocks are accepted, red blocks are removed time.' : `Analysis is ${data.job.phase.replaceAll('_', ' ')}.`}</Card.Description>
					</Card.Header>
					<Card.Content class="space-y-4">
						{#if timelineItems.length > 0}
							<div class="space-y-2">
								<div bind:this={timelineElement} class="relative pt-3">
									<div class="absolute top-0 bottom-0 z-10 -translate-x-1/2" style={`left: ${playheadPercent}%`}>
										<button
											type="button"
											class="mx-auto block h-0 w-0 cursor-ew-resize border-x-[8px] border-t-[12px] border-x-transparent border-t-red-500 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
											onpointerdown={startPlayheadDrag}
											onpointermove={dragPlayhead}
											onpointerup={endPlayheadDrag}
											onpointercancel={endPlayheadDrag}
											aria-label="Drag preview playhead"
										></button>
										<div class="pointer-events-none mx-auto h-[5rem] w-0.5 bg-red-500 shadow-[0_0_0_1px_rgba(255,255,255,0.55)]"></div>
									</div>
									<div class="flex h-20 w-full overflow-hidden rounded-md">
									{#each timelineItems as item, index (item.id)}
										<button
											type="button"
											class={[item.included ? 'bg-foreground hover:bg-foreground/90' : 'bg-destructive/25 hover:bg-destructive/35', 'h-full min-w-1 transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'].join(' ')}
											style={`width: ${itemWidth(item.start, item.end)}`}
											title={`${item.included ? 'Accepted' : 'Removed'} ${formatTime(item.start)} to ${formatTime(item.end)}`}
											onclick={() => seekToSegment(item.start)}
											aria-label={`Jump to ${item.included ? 'accepted segment' : 'removed time'} ${index + 1} at ${formatTime(item.start)}`}
										></button>
									{/each}
									</div>
								</div>
								<div class="flex justify-between text-xs text-muted-foreground">
									<span>0:00</span>
									<span>{formatTime(duration)}</span>
								</div>
							</div>

							<div class="flex flex-wrap gap-2 text-sm">
								<span class="rounded-full border px-3 py-1">{acceptedCount} accepted</span>
								<span class="rounded-full border px-3 py-1">{removedCount} removed gaps</span>
								<span class="rounded-full border px-3 py-1">{formatTime(duration)} source</span>
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
			</div>

			<Card.Root class="shadow-sm lg:sticky lg:top-6 lg:self-start">
				<Card.Header>
					<Card.Title>Transcript</Card.Title>
					<Card.Description>{ready ? 'Click any word to seek the preview there.' : 'Analysis is still running.'}</Card.Description>
				</Card.Header>
				<Card.Content>
					{#if transcriptWords.length > 0}
						<div class="max-h-[42rem] overflow-y-auto text-sm leading-8">
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
						<p class="text-muted-foreground max-h-[42rem] overflow-y-auto text-sm leading-7 whitespace-pre-wrap">{transcript}</p>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>
	</section>
</main>
