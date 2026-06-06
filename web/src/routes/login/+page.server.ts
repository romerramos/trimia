import { auth } from '$lib/server/auth';
import { fail, redirect } from '@sveltejs/kit';
import { APIError } from 'better-auth/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = (event) => {
	if (event.locals.user) {
		return redirect(302, '/upload');
	}
	return {};
};

export const actions: Actions = {
	signInEmail: async (event) => {
		const formData = await event.request.formData();
		const email = formData.get('email')?.toString() ?? '';
		const password = formData.get('password')?.toString() ?? '';

		try {
			await auth.api.signInEmail({
				body: {
					email,
					password,
					callbackURL: '/upload'
				}
			});
		} catch (error) {
			if (error instanceof APIError) {
				return fail(400, { message: error.message || 'Signin failed' });
			}

			console.error('Unexpected sign-in error', error);
			return fail(500, { message: error instanceof Error ? error.message : 'Unexpected error' });
		}

		return redirect(302, '/upload');
	},
	signUpEmail: async (event) => {
		const formData = await event.request.formData();
		const email = formData.get('email')?.toString() ?? '';
		const password = formData.get('password')?.toString() ?? '';
		const confirmPassword = formData.get('confirmPassword')?.toString() ?? '';
		const name = formData.get('name')?.toString() ?? '';

		if (password !== confirmPassword) {
			return fail(400, { message: 'Passwords do not match' });
		}

		if (password.length < 8) {
			return fail(400, { message: 'Password must be at least 8 characters long' });
		}

		try {
			await auth.api.signUpEmail({
				body: {
					email,
					password,
					name,
					callbackURL: '/upload'
				}
			});
		} catch (error) {
			if (error instanceof APIError) {
				return fail(400, { message: error.message || 'Registration failed' });
			}

			console.error('Unexpected registration error', error);
			return fail(500, { message: error instanceof Error ? error.message : 'Unexpected error' });
		}

		return redirect(302, '/upload');
	}
};
