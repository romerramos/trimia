<script lang="ts">
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
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
	let uploadStatus = $state('');
	let createProxy = $state(false);
	let media = $state<core.Media>();

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
		uploadStatus = '';
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
			uploadStatus = 'Uploading video';
			const uploadedMedia = await uploadWithProgress(
				data.upload.url,
				data.upload.token,
				file,
				createProxy
			);
			media = uploadedMedia;
			progress = 100;
			if (createProxy) {
				media = await waitForPreview(uploadedMedia.mediaId);
			}
			uploadStatus = 'Starting analysis';
			const job = await createJob(data.jobsUrl, uploadedMedia.mediaId);
			await goto(resolve('/studio/[jobId]', { jobId: job.jobId }));
		} catch (uploadError) {
			error = 'Upload failed.';
			detail = uploadError instanceof Error ? uploadError.message : 'Unknown upload error';
		} finally {
			uploading = false;
		}
	};

	const uploadWithProgress = (uploadUrl: string, token: string, file: File, proxy: boolean) => {
		return new Promise<core.Media>((resolve, reject) => {
			const request = new XMLHttpRequest();
			const body = new FormData();
			body.append('file', file, file.name);
			const url = new URL(uploadUrl);
			url.searchParams.set('proxy', proxy ? '1' : '0');

			request.open('POST', url.toString());
			request.setRequestHeader('Authorization', `Bearer ${token}`);
			request.upload.onprogress = (event) => {
				if (event.lengthComputable) {
					progress = Math.round((event.loaded / event.total) * 100);
					if (proxy && progress >= 100) {
						uploadStatus = 'Preparing browser preview';
					}
				}
			};
			request.onload = () => {
				if (request.status >= 200 && request.status < 300) {
					resolve(JSON.parse(request.responseText) as core.Media);
					return;
				}

				reject(new Error(request.responseText || `Trimia returned ${request.status}`));
			};
			request.onerror = () => reject(new Error('Could not reach the Trimia server.'));
			request.send(body);
		});
	};

	const waitForPreview = async (mediaId: string) => {
		const startedAt = Date.now();
		const timeoutMs = 20 * 60 * 1000;

		while (Date.now() - startedAt < timeoutMs) {
			const nextMedia = await fetchMedia(data.upload.url, mediaId);
			progress = Math.round(nextMedia.previewProgress ?? 0);
			uploadStatus = 'Preparing browser preview';

			if (nextMedia.previewStatus === 'preview_ready') {
				progress = 100;
				return nextMedia;
			}

			if (nextMedia.previewStatus === 'preview_failed') {
				throw new Error(nextMedia.previewError || 'Could not prepare browser preview.');
			}

			await new Promise((resolve) => window.setTimeout(resolve, 1000));
		}

		throw new Error('Preparing the browser preview timed out.');
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

	const fetchMedia = async (mediaUrl: string, mediaId: string) => {
		const response = await fetch(`${mediaUrl.replace(/\/$/, '')}/${mediaId}`);
		if (!response.ok) {
			throw new Error((await response.text()) || `Trimia returned ${response.status}`);
		}

		return (await response.json()) as core.Media;
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
				Send an MP4, MOV, or WebM file to the local Trimia server.
			</p>
		</div>

		<Card.Root class="shadow-sm">
			<Card.Header>
				<Card.Description>
					Large files are supported (max 5GB). Keep this tab open while Trimia uploads and starts
					analysis.
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<form class="space-y-5" onsubmit={uploadVideo}>
					<div class="space-y-2">
						<Input
							id="file"
							name="file"
							type="file"
							accept="video/mp4,video/quicktime,video/webm"
							bind:files
						/>
					</div>

					<div class="flex items-start gap-3 rounded-lg border bg-muted/30 p-3">
						<Checkbox id="create-proxy" bind:checked={createProxy} class="mt-0.5" />
						<div class="space-y-1 leading-none">
							<Label for="create-proxy">Create faster Studio preview</Label>
							<p class="text-sm leading-5 text-muted-foreground">
								Creates a lower-resolution preview for the Studio player, which can make cuts and
								scrubbing feel snappier. Leave it unchecked to skip the extra wait.
							</p>
							<p class="text-sm leading-5 text-muted-foreground">
								Final renders still use the original video quality.
							</p>
						</div>
					</div>

					{#if uploading}
						<div class="space-y-2">
							<div class="h-2 overflow-hidden rounded-full bg-muted">
								<div
									class="h-full rounded-full bg-primary transition-all"
									style={`width: ${progress}%`}
								></div>
							</div>
							<p class="text-sm text-muted-foreground">
								{uploadStatus || 'Uploading video'}
								{progress}%
							</p>
						</div>
					{/if}

					<Button type="submit" disabled={uploading}
						>{uploading ? uploadStatus || 'Uploading...' : 'Upload video'}</Button
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
