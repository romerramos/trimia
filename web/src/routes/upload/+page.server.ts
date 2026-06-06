import { env } from '$env/dynamic/private';
import { redirect } from '@sveltejs/kit';
import { createHmac } from 'node:crypto';
import type { PageServerLoad } from './$types';

const trimiaApiUrl = env.TRIMIA_API_URL ?? 'http://127.0.0.1:3333';
const tokenSecret = env.TRIMIA_UPLOAD_TOKEN_SECRET ?? env.BETTER_AUTH_SECRET;

export const load: PageServerLoad = (event) => {
	if (!event.locals.user) {
		return redirect(302, '/login');
	}

	return {
		user: event.locals.user,
		upload: {
			url: new URL('/api/media', trimiaApiUrl).toString(),
			token: createUploadJwt(event.locals.user.id)
		},
		jobsUrl: new URL('/api/jobs', trimiaApiUrl).toString()
	};
};

function createUploadJwt(userId: string) {
	if (!tokenSecret) {
		throw new Error('TRIMIA_UPLOAD_TOKEN_SECRET or BETTER_AUTH_SECRET is required.');
	}

	const now = Math.floor(Date.now() / 1000);
	const header = base64Url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
	const payload = base64Url(
		JSON.stringify({
			sub: userId,
			aud: 'trimia-upload',
			scope: 'media:upload',
			iat: now,
			exp: now + 60 * 60
		})
	);
	const unsignedToken = `${header}.${payload}`;
	const signature = createHmac('sha256', tokenSecret).update(unsignedToken).digest('base64url');

	return `${unsignedToken}.${signature}`;
}

function base64Url(value: string) {
	return Buffer.from(value).toString('base64url');
}
