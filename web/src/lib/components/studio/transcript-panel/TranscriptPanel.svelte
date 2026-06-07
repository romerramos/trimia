<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { formatTime } from '../helpers';
	import type { TranscriptWord } from './types';

	let {
		words,
		activeWordId,
		fallbackTranscript,
		onSeek
	}: {
		words: TranscriptWord[];
		activeWordId: string;
		fallbackTranscript: string;
		onSeek: (time: number) => void;
	} = $props();

	let scrollElement = $state<HTMLDivElement>();

	$effect(() => {
		if (!activeWordId || !scrollElement) {
			return;
		}

		const element = scrollElement.querySelector<HTMLElement>(`[data-word-id="${activeWordId}"]`);
		element?.scrollIntoView({ block: 'center', behavior: 'smooth' });
	});
</script>

<Card.Root class="min-h-0 overflow-hidden shadow-sm">
	<Card.Header>
		<Card.Title>Transcript</Card.Title>
	</Card.Header>
	<Card.Content class="min-h-0 pb-4">
		{#if words.length > 0}
			<div
				bind:this={scrollElement}
				class="max-h-[calc(100dvh-16rem)] overflow-y-auto pr-1 text-sm leading-8"
			>
				{#each words as word (word.id)}
					<button
						type="button"
						data-word-id={word.id}
						class={[
							word.id === activeWordId
								? 'bg-primary text-primary-foreground'
								: 'text-muted-foreground hover:bg-muted hover:text-foreground',
							'inline rounded px-1 py-0.5 text-left transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
						].join(' ')}
						onclick={() => onSeek(word.start)}
						aria-label={`Jump to ${word.text} at ${formatTime(word.start)}`}
					>
						{word.text}
					</button>
				{/each}
			</div>
		{:else}
			<p
				class="max-h-[36rem] overflow-y-auto text-sm leading-7 whitespace-pre-wrap text-muted-foreground"
			>
				{fallbackTranscript}
			</p>
		{/if}
	</Card.Content>
</Card.Root>
