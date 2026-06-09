<script lang="ts">
	import { Pause, Play, Square } from '@lucide/svelte';
	import { formatTime } from '../helpers';

	let {
		player = $bindable(),
		sourceUrl,
		ready,
		disabled,
		playing,
		currentSourceTime,
		duration,
		onTimeUpdate,
		onToggle,
		onStop
	}: {
		player?: HTMLVideoElement;
		sourceUrl: string;
		ready: boolean;
		disabled: boolean;
		playing: boolean;
		currentSourceTime: number;
		duration: number;
		onTimeUpdate: (time: number) => void;
		onToggle: () => void;
		onStop: () => void;
	} = $props();
</script>

<div class="flex min-h-0 items-stretch">
	<div class="flex h-full max-h-full w-full flex-col overflow-hidden rounded-xl bg-black shadow-sm">
		<video
			bind:this={player}
			class="min-h-0 flex-1 bg-black object-contain"
			src={sourceUrl}
			preload="metadata"
			ontimeupdate={() => onTimeUpdate(player?.currentTime ?? 0)}
			onended={onStop}
		>
			<track kind="captions" />
		</video>

		<div
			class="flex shrink-0 items-center justify-between gap-4 border-t border-white/10 bg-zinc-950 px-4 py-3 text-white"
		>
			<div class="flex items-center gap-2">
				<button
					type="button"
					class="flex h-10 w-10 items-center justify-center rounded-full bg-white text-black transition hover:bg-white/90 disabled:cursor-not-allowed disabled:opacity-40"
					disabled={!ready || disabled}
					onclick={onToggle}
					aria-label={playing ? 'Pause preview' : 'Play preview'}
				>
					{#if playing}
						<Pause class="h-5 w-5" />
					{:else}
						<Play class="h-5 w-5" />
					{/if}
				</button>
				<button
					type="button"
					class="flex h-10 w-10 items-center justify-center rounded-full border border-white/20 text-white transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-40"
					disabled={!ready || disabled}
					onclick={onStop}
					aria-label="Stop preview"
				>
					<Square class="h-4 w-4" />
				</button>
			</div>

			<div class="font-mono text-sm text-zinc-300 tabular-nums">
				{formatTime(currentSourceTime)} / {formatTime(duration)}
			</div>
		</div>
	</div>
</div>
