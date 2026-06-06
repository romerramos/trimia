<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
</script>

<svelte:head>
	<title>Trimia</title>
</svelte:head>

<main class="bg-background min-h-screen overflow-hidden px-6 py-10">
	<section class="relative mx-auto flex min-h-[calc(100vh-5rem)] w-full max-w-6xl items-center">
		<div class="bg-muted absolute -top-24 right-0 size-72 rounded-full blur-3xl"></div>
		<div class="bg-primary/10 absolute bottom-8 left-12 size-80 rounded-full blur-3xl"></div>

		<div class="relative grid w-full gap-12 lg:grid-cols-[1fr_24rem] lg:items-center">
			<div class="space-y-8">
				<div class="border-border bg-card/70 inline-flex items-center rounded-full border px-3 py-1 text-sm shadow-sm backdrop-blur">
					<span class="bg-primary mr-2 size-2 rounded-full"></span>
					Coming soon
				</div>

				<div class="space-y-5">
					<h1 class="max-w-3xl text-5xl font-semibold tracking-tight text-balance sm:text-7xl">
						Video cleanup that starts before your editor opens.
					</h1>
					<p class="text-muted-foreground max-w-2xl text-lg text-pretty sm:text-xl">
						Trimia removes silence and filler words from raw recordings so every cut starts from a cleaner timeline.
					</p>
				</div>

				<div class="flex flex-col gap-3 sm:flex-row">
					<Button href="/upload" size="lg">Start now</Button>
					<Button href="/login" variant="outline" size="lg">Sign in</Button>
				</div>
			</div>

			<div class="border-border bg-card/80 relative rounded-3xl border p-5 shadow-2xl backdrop-blur">
				<div class="space-y-4 rounded-2xl border p-4">
					<div class="flex items-center justify-between text-sm">
						<span class="font-medium">demo_recording.mov</span>
						<span class="text-muted-foreground">12:48</span>
					</div>
					<div class="bg-muted h-3 overflow-hidden rounded-full">
						<div class="bg-primary h-full w-2/3 rounded-full"></div>
					</div>
					<div class="grid grid-cols-3 gap-2 text-center text-sm">
						<div class="bg-muted rounded-xl p-3"><p class="font-semibold">31%</p><p class="text-muted-foreground">Removed</p></div>
						<div class="bg-muted rounded-xl p-3"><p class="font-semibold">42</p><p class="text-muted-foreground">Cuts</p></div>
						<div class="bg-muted rounded-xl p-3"><p class="font-semibold">8:51</p><p class="text-muted-foreground">Output</p></div>
					</div>
				</div>
			</div>
		</div>
	</section>
</main>
