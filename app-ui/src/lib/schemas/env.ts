// Panel environment schema.
//
// Pure and dependency-free so it can be unit tested: the server module that reads the real
// environment calls parseEnv with a plain record (src/lib/server/config.ts).

import { z } from 'zod';
import { absoluteUrl } from './primitives';

export const envSchema = z.strictObject({
	PANEL_API_TARGET: absoluteUrl
});

export type PanelEnv = z.infer<typeof envSchema>;

export class EnvError extends Error {
	constructor(
		readonly variable: string,
		reason: string
	) {
		super(`${variable}: ${reason}`);
		this.name = 'EnvError';
	}
}

export function parseEnv(source: Record<string, string | undefined>): PanelEnv {
	// The process environment holds every variable, not just ours, so only the declared keys are
	// selected before the strict parse. Strictness then applies where it is useful: a missing or
	// malformed panel variable, and a PANEL_ prefix that matches nothing declared.
	const selected: Record<string, string | undefined> = {};
	for (const key of Object.keys(envSchema.shape)) selected[key] = source[key];

	for (const key of Object.keys(source)) {
		if (key.startsWith('PANEL_') && !(key in envSchema.shape)) {
			throw new EnvError(key, 'Unknown panel variable. Remove it or add it to envSchema.');
		}
	}

	const parsed = envSchema.safeParse(selected);

	if (!parsed.success) {
		const first = parsed.error.issues[0];
		throw new EnvError(first.path.join('.') || 'env', first.message);
	}

	return parsed.data;
}
