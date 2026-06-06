<script lang="ts">
	import { enhance } from '$app/forms';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Field, FieldDescription, FieldGroup, FieldLabel } from '$lib/components/ui/field/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import type { ActionData } from './$types';

	let { form }: { form?: ActionData } = $props();
</script>

<svelte:head>
	<title>Sign in | Trimia</title>
</svelte:head>

<main class="bg-background min-h-screen px-6 py-12">
	<section class="mx-auto grid min-h-[calc(100vh-6rem)] w-full max-w-5xl items-center gap-10 lg:grid-cols-[1fr_25rem]">
		<div class="hidden space-y-6 lg:block">
			<div class="bg-primary text-primary-foreground flex size-12 items-center justify-center rounded-2xl text-lg font-semibold shadow-sm">
				T
			</div>
			<div class="space-y-4">
				<p class="text-muted-foreground text-sm font-medium uppercase tracking-[0.24em]">Trimia studio</p>
				<h1 class="max-w-xl text-5xl font-semibold tracking-tight text-balance">Cut the dead air before it reaches the timeline.</h1>
				<p class="text-muted-foreground max-w-lg text-lg text-pretty">
					Sign in to upload videos, send them to the local Trimia server, and start building a cleaner edit flow.
				</p>
			</div>
			<div class="border-border/70 grid max-w-lg grid-cols-3 gap-3 border-t pt-6 text-sm">
				<div><p class="font-medium">Upload</p><p class="text-muted-foreground">MP4, MOV, WebM</p></div>
				<div><p class="font-medium">Analyze</p><p class="text-muted-foreground">Silence and fillers</p></div>
				<div><p class="font-medium">Review</p><p class="text-muted-foreground">Keep control</p></div>
			</div>
		</div>

		<div class="mx-auto w-full max-w-sm space-y-4">
			<Card.Root>
				<Card.Header>
					<Card.Title class="text-2xl">Welcome back</Card.Title>
					<Card.Description>Use your email and password to continue to Trimia.</Card.Description>
				</Card.Header>
				<Card.Content>
					<form method="post" action="?/signInEmail" use:enhance>
						<FieldGroup>
							<Field>
								<FieldLabel for="signin-email">Email</FieldLabel>
								<Input id="signin-email" name="email" type="email" placeholder="you@example.com" autocomplete="email" required />
							</Field>
							<Field>
								<FieldLabel for="signin-password">Password</FieldLabel>
								<Input id="signin-password" name="password" type="password" autocomplete="current-password" required />
							</Field>
							<Field><Button type="submit" class="w-full">Sign in</Button></Field>
						</FieldGroup>
					</form>

					<Separator class="my-6" />

					<form method="post" action="?/signUpEmail" use:enhance>
						<FieldGroup>
							<Field>
								<FieldLabel for="signup-name">Create an account</FieldLabel>
								<Input id="signup-name" name="name" placeholder="Your name" autocomplete="name" required />
							</Field>
							<Field>
								<FieldLabel for="signup-email">Email</FieldLabel>
								<Input id="signup-email" name="email" type="email" placeholder="you@example.com" autocomplete="email" required />
							</Field>
							<Field>
								<FieldLabel for="signup-password">Password</FieldLabel>
								<Input id="signup-password" name="password" type="password" autocomplete="new-password" required />
								<FieldDescription>Use at least 8 characters.</FieldDescription>
							</Field>
							<Field>
								<FieldLabel for="signup-confirm-password">Confirm password</FieldLabel>
								<Input id="signup-confirm-password" name="confirmPassword" type="password" autocomplete="new-password" required />
							</Field>
							<Field><Button type="submit" variant="outline" class="w-full">Create account</Button></Field>
						</FieldGroup>
					</form>
				</Card.Content>
			</Card.Root>

			{#if form?.message}
				<Alert variant="destructive">
					<AlertTitle>Authentication failed</AlertTitle>
					<AlertDescription>{form.message}</AlertDescription>
				</Alert>
			{/if}
		</div>
	</section>
</main>
