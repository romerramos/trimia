<script lang="ts">
	import type { TimelineZoom } from './types';

	let {
		zoom,
		onChange
	}: {
		zoom: TimelineZoom;
		onChange: (zoom: TimelineZoom) => void;
	} = $props();
</script>

<div class="flex rounded-lg bg-muted p-1">
	<button
		type="button"
		class={[
			zoom === 'fit'
				? 'bg-background text-foreground shadow-sm'
				: 'text-muted-foreground hover:text-foreground',
			'rounded-md px-2 py-1 text-xs font-medium transition'
		].join(' ')}
		onclick={() => onChange('fit')}
	>
		Full width
	</button>
	<button
		type="button"
		class={[
			zoom === 'playhead'
				? 'bg-background text-foreground shadow-sm'
				: 'text-muted-foreground hover:text-foreground',
			'rounded-md px-2 py-1 text-xs font-medium transition'
		].join(' ')}
		onclick={() => onChange('playhead')}
	>
		Zoom on playhead
	</button>
</div>
