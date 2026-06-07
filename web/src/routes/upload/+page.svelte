<script lang="ts">
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type * as core from '$lib/concepts/core';
	import type { PageServerData } from './$types';

	let { data }: { data: PageServerData } = $props();
	let files = $state<FileList>();
	let uploading = $state(false);
	let progress = $state(0);
	let error = $state('');
	let detail = $state('');
	let media = $state<TrimiaMedia>();

	type TrimiaMedia = {
		mediaId: string;
		filename: string;
		contentType: string;
		sizeBytes: number;
		durationSeconds: number;
		status: string;
		createdAt: string;
	};

	const formatBytes = (bytes: number) => {
		const units = ['B', 'KB', 'MB', 'GB'];
		let size = bytes;
		let unit = 0;

		while (size >= 1024 && unit < units.length - 1) {
			size /= 1024;
			unit += 1;
		}

		return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
	};

	const formatDuration = (seconds: number) => {
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = Math.round(seconds % 60);
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	};

	const uploadVideo = async (event: SubmitEvent) => {
		event.preventDefault();
		error = '';
		detail = '';
		media = undefined;
		progress = 0;

		const file = files?.item(0);
		if (!file) {
			error = 'Choose a video file to upload.';
			return;
		}

		if (!['video/mp4', 'video/quicktime', 'video/webm'].includes(file.type)) {
			error = 'Upload an MP4, MOV, or WebM video.';
			return;
		}

		uploading = true;
		try {
			media = await uploadWithProgress(data.upload.url, data.upload.token, file);
			progress = 100;
			const job = await createJob(data.jobsUrl, media.mediaId);
			await goto(resolve('/studio/[jobId]', { jobId: job.jobId }));
		} catch (uploadError) {
			error = 'Upload failed.';
			detail = uploadError instanceof Error ? uploadError.message : 'Unknown upload error';
		} finally {
			uploading = false;
		}
	};

	const uploadWithProgress = (uploadUrl: string, token: string, file: File) => {
		return new Promise<TrimiaMedia>((resolve, reject) => {
			const request = new XMLHttpRequest();
			const body = new FormData();
			body.append('file', file, file.name);

			request.open('POST', uploadUrl);
			request.setRequestHeader('Authorization', `Bearer ${token}`);
			request.upload.onprogress = (event) => {
				if (event.lengthComputable) {
					progress = Math.round((event.loaded / event.total) * 100);
				}
			};
			request.onload = () => {
				if (request.status >= 200 && request.status < 300) {
					resolve(JSON.parse(request.responseText) as TrimiaMedia);
					return;
				}

				reject(new Error(request.responseText || `Trimia returned ${request.status}`));
			};
			request.onerror = () => reject(new Error('Could not reach the Trimia server.'));
			request.send(body);
		});
	};

	const createJob = async (jobsUrl: string, mediaId: string) => {
		const response = await fetch(jobsUrl, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				mediaId,
				options: {
					removeSilence: true,
					removeFillerWords: true,
					detectLanguage: true
				}
			})
		});

		if (!response.ok) {
			throw new Error((await response.text()) || `Trimia returned ${response.status}`);
		}

		return (await response.json()) as core.Job;
	};
</script>

<svelte:head>
	<title>Upload video | Trimia</title>
</svelte:head>

<main
	class="min-h-screen bg-[radial-gradient(circle_at_top_left,var(--muted),transparent_34rem)] px-6 py-12"
>
	<section class="mx-auto flex w-full max-w-3xl flex-col gap-8">
		<div class="space-y-3">
			<p class="text-sm font-medium text-muted-foreground">Signed in as {data.user.email}</p>
			<h1 class="text-4xl font-semibold tracking-tight text-balance">Upload a video to Trimia</h1>
			<p class="max-w-2xl text-lg text-pretty text-muted-foreground">
				Send an MP4, MOV, or WebM file to the local Trimia server. Trimia will save the upload and
				probe the duration before the next editing step.
			</p>
		</div>

		<Card.Root class="shadow-sm">
			<Card.Header>
				<Card.Title>Video upload</Card.Title>
				<Card.Description
					>Large files are supported. Keep this tab open while Trimia uploads and starts analysis.</Card.Description
				>
			</Card.Header>
			<Card.Content>
				<form class="space-y-5" onsubmit={uploadVideo}>
					<div class="space-y-2">
						<Label for="file">Video file</Label>
						<Input
							id="file"
							name="file"
							type="file"
							accept="video/mp4,video/quicktime,video/webm"
							bind:files
						/>
						<p class="text-sm text-muted-foreground">
							The browser uploads directly to Trimia with a short-lived upload token, so SvelteKit
							does not buffer the video.
						</p>
					</div>

					{#if uploading}
						<div class="space-y-2">
							<div class="h-2 overflow-hidden rounded-full bg-muted">
								<div
									class="h-full rounded-full bg-primary transition-all"
									style={`width: ${progress}%`}
								></div>
							</div>
							<p class="text-sm text-muted-foreground">Uploading {progress}%</p>
						</div>
					{/if}

					<Button type="submit" disabled={uploading}
						>{uploading ? 'Uploading...' : 'Upload video'}</Button
					>
				</form>
			</Card.Content>
		</Card.Root>

		{#if error}
			<Alert variant="destructive">
				<AlertTitle>Upload failed</AlertTitle>
				<AlertDescription>{error}{detail ? ` ${detail}` : ''}</AlertDescription>
			</Alert>
		{/if}

		{#if media}
			<Alert>
				<AlertTitle>Video uploaded</AlertTitle>
				<AlertDescription>
					<span class="block">{media.filename}</span>
					<span class="block text-sm text-muted-foreground">
						Media ID {media.mediaId} · {formatDuration(media.durationSeconds)} · {formatBytes(
							media.sizeBytes
						)}
					</span>
				</AlertDescription>
			</Alert>
		{/if}
	</section>
</main>
