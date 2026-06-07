<script lang="ts">
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { formatTime } from '../helpers';
	import { contextMenuLabel, itemClass } from './helpers';
	import type { TimelineItem } from './types';

	let {
		item,
		index,
		width,
		savingSegmentId,
		onSeek,
		onAddGap,
		onToggleSegment
	}: {
		item: TimelineItem;
		index: number;
		width: string;
		savingSegmentId: string;
		onSeek: (time: number) => void;
		onAddGap: (item: TimelineItem) => void;
		onToggleSegment: (segmentId: string, included: boolean) => void;
	} = $props();
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger style={`width: ${width}; flex: 0 0 ${width}`} class="h-full">
		<button
			type="button"
			class={itemClass(item, savingSegmentId)}
			title={`${item.included ? 'Accepted' : 'Removed'} ${formatTime(item.start)} to ${formatTime(item.end)}`}
			onclick={() => onSeek(item.start)}
			aria-label={`Jump to ${item.included ? 'accepted segment' : 'removed time'} ${index + 1} at ${formatTime(item.start)}`}
		></button>
	</ContextMenu.Trigger>
	<ContextMenu.Content
		class="z-50 min-w-44 rounded-lg border bg-popover p-1 text-popover-foreground shadow-md"
	>
		<ContextMenu.Item
			class="relative flex cursor-default items-center rounded-sm px-2 py-1.5 text-sm transition-colors outline-none select-none hover:bg-accent hover:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50"
			disabled={!!item.segmentId && savingSegmentId === item.segmentId}
			onSelect={() => {
				if (item.kind === 'gap') {
					onAddGap(item);
				} else if (item.segmentId) {
					onToggleSegment(item.segmentId, !item.included);
				}
			}}
		>
			{contextMenuLabel(item)}
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
