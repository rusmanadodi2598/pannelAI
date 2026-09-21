// Panel environment schema.
//
// Pure and dependency-free so it can be unit tested: the server module that reads the real
// environment calls parseEnv with a plain record (src/lib/server/config.ts).

import { z } from 'zod';
import { absoluteUrl } from './primitives';

/**
 * The shortest value the panel accepts as a gateway key.
 *
 * A gateway key is `prefix + 48` random characters (app-serv `internal/service/gateway_key.go`), so the
 * real value is far longer than this. The floor exists to catch a placeholder or a truncated paste, not
 * to describe the key's format: the prefix is configurable on the gateway side, so the panel must not
 * pin one.
 */
const MIN_GATEWAY_KEY_LENGTH = 8;

/**
 * A gateway key as it arrives from the environment.
 *
 * Two refusals are worth the lines. A value carrying whitespace is a paste that picked up a newline or
 * a space, and a value carrying the `Bearer ` prefix is an operator who copied the header instead of
 * the key; both would reach the gateway as a 401 with nothing on screen pointing at the environment.
 */
const gatewayKey = z
	.string()
	.trim()
	.min(
		MIN_GATEWAY_KEY_LENGTH,
		`must be a gateway key of at least ${MIN_GATEWAY_KEY_LENGTH} characters`
	)
	.refine((value) => !/\s/.test(value), 'must not contain whitespace')
	.refine(
		(value) => !/^bearer\b/i.test(value),
		'must be the key alone: the panel adds the Bearer prefix'
	);

/**
 * The playground key, absent or valid.
 *
 * Absent is a supported state rather than a boot failure: SPEC-UI §6.15 requires the screen to say the
 * panel has no gateway key configured, and a panel that refused to start would take every other screen
 * down with it. A blank value means absent, because that is what an untouched line in a `.env` file
 * looks like, and treating it as a malformed key would fail a boot for a variable nobody set.
 */
const optionalGatewayKey = z.preprocess(
	(value) => (typeof value === 'string' && value.trim() === '' ? undefined : value),
	gatewayKey.optional()
);

export const envSchema = z.strictObject({
	PANEL_API_TARGET: absoluteUrl,
	PANEL_PLAYGROUND_KEY: optionalGatewayKey
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
