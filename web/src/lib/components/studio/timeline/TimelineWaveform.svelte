<script lang="ts">
	import type * as core from '$lib/concepts/core';
	import { onDestroy, onMount } from 'svelte';
	import type WaveSurfer from 'wavesurfer.js';

	let {
		waveform,
		duration
	}: {
		waveform?: core.Waveform;
		duration: number;
	} = $props();

	let container = $state<HTMLDivElement>();
	let wavesurfer = $state<WaveSurfer>();
	let loadError = $state('');

	$effect(() => {
		void loadWaveform();
	});

	async function loadWaveform() {
		if (!wavesurfer || !waveform || duration <= 0) {
			return;
		}

		try {
			loadError = '';
			await wavesurfer.load('', waveform.peaks, waveform.durationSeconds || duration);
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'Could not render waveform.';
		}
	}

	onMount(async () => {
		if (!container) {
			return;
		}
		const { default: WaveSurfer } = await import('wavesurfer.js');

		wavesurfer = WaveSurfer.create({
			container,
			height: 64,
			interact: false,
			cursorWidth: 0,
			normalize: true,
			waveColor: 'rgba(24, 24, 27, 0.38)',
			progressColor: 'rgba(24, 24, 27, 0.38)',
			barWidth: 1,
			barGap: 1,
			barRadius: 1,
			hideScrollbar: true
		});
	});

	onDestroy(() => {
		wavesurfer?.destroy();
	});
</script>

<div class="pointer-events-none absolute inset-0 z-0 opacity-85">
	<div bind:this={container} class="h-full w-full"></div>
	{#if !waveform || duration <= 0}
		<div class="absolute inset-0 flex items-center px-1">
			<div class="h-px w-full bg-foreground/20"></div>
		</div>
	{:else if loadError}
		<div class="absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">
			Waveform unavailable
		</div>
	{/if}
</div>
