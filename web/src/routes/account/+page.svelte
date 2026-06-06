<script lang="ts">
	import { enhance } from '$app/forms';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import type { PageServerData } from './$types';

	let { data }: { data: PageServerData } = $props();
</script>

<svelte:head>
	<title>Account | Trimia</title>
</svelte:head>

<main class="bg-background min-h-screen px-6 py-12">
	<section class="mx-auto flex min-h-[calc(100vh-6rem)] w-full max-w-2xl items-center">
		<Card.Root class="w-full">
			<Card.Header>
				<Card.Title>Hi, {data.user.name}.</Card.Title>
				<Card.Description>You are signed in and ready to upload videos to Trimia.</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-4">
				<div class="bg-muted/60 rounded-lg border p-4 text-sm">
					<p class="text-muted-foreground">User ID</p>
					<p class="font-mono break-all">{data.user.id}</p>
				</div>
				<div class="flex flex-col gap-3 sm:flex-row">
					<Button href="/upload" class="sm:w-auto">Upload a video</Button>
					<form method="post" action="?/signOut" use:enhance>
						<Button type="submit" variant="outline" class="w-full sm:w-auto">Sign out</Button>
					</form>
				</div>
			</Card.Content>
		</Card.Root>
	</section>
</main>
