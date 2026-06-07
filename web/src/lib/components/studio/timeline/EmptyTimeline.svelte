<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';

	let { onRefresh }: { onRefresh: () => void } = $props();
</script>

<div class="rounded-xl border border-dashed bg-muted/60 p-8 text-center">
	<p class="font-medium">Timeline is waiting for analysis.</p>
	<p class="mt-2 text-sm text-muted-foreground">
		Trimia will add segments here as soon as the transcript analysis completes.
	</p>
	<Button class="mt-4" variant="outline" onclick={onRefresh}>Refresh</Button>
</div>
