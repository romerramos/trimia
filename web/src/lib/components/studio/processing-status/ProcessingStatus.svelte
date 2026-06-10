<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { formatTime } from '../helpers';

	let {
		phase,
		progress,
		indeterminate,
		elapsedSeconds
	}: {
		phase: string;
		progress: number;
		indeterminate: boolean;
		elapsedSeconds: number;
	} = $props();
</script>

<Card.Root class="shadow-sm">
	<Card.Content class="pt-6">
		<div class="space-y-3">
			<div class="flex justify-between gap-4 text-sm">
				<span class="font-medium capitalize">{phase.replaceAll('_', ' ')}</span>
				{#if indeterminate}
					<span class="font-mono text-muted-foreground tabular-nums"
						>Working… {formatTime(elapsedSeconds)}</span
					>
				{:else}
					<span class="text-muted-foreground">{Math.round(progress)}%</span>
				{/if}
			</div>
			<div class="h-2 overflow-hidden rounded-full bg-muted">
				{#if indeterminate}
					<div
						class="h-full animate-[transcribe-shimmer_1.4s_linear_infinite] rounded-full bg-gradient-to-r from-muted via-primary to-muted bg-[length:200%_100%]"
					></div>
				{:else}
					<div
						class="h-full rounded-full bg-primary transition-all duration-500"
						style={`width: ${Math.max(progress, 4)}%`}
					></div>
				{/if}
			</div>
			{#if indeterminate}
				<p class="text-xs text-muted-foreground">
					Transcribing audio… this may take a while.
				</p>
			{/if}
		</div>
	</Card.Content>
</Card.Root>

<style>
	@keyframes transcribe-shimmer {
		0% {
			background-position: 100% 0;
		}
		100% {
			background-position: -100% 0;
		}
	}
</style>
