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
 * Absent-or-blank, and trimmed, for every variable the env template ships with a default.
 *
 * A blank value means absent, because that is what an untouched line in a `.env` file looks like,
 * and treating it as a malformed value would fail a boot for a variable nobody set.
 */
function blankAsAbsent(value: unknown): unknown {
	if (typeof value !== 'string') return value;
	const trimmed = value.trim();
	return trimmed === '' ? undefined : trimmed;
}

/**
 * The playground key, absent or valid.
 *
 * Absent is a supported state rather than a boot failure: SPEC-UI §6.15 requires the screen to say the
 * panel has no gateway key configured, and a panel that refused to start would take every other screen
 * down with it.
 */
const optionalGatewayKey = z.preprocess(blankAsAbsent, gatewayKey.optional());

// The panel's runtime environment, as the monorepo's env template names it. A closed set rather than
// a free string: SPEC-UI §10.1 item 5 has the boot banner state what is running, so a typo would
// otherwise be printed as a fourth environment nobody meant.
const PANEL_ENVIRONMENTS = ['development', 'production', 'test'] as const;

const appEnv = z.preprocess(
	blankAsAbsent,
	z
		.enum(PANEL_ENVIRONMENTS, { message: 'must be development, production, or test' })
		.default('development')
);

// The port the built panel server binds. adapter-node reads `PORT` and the env template calls it
// `APP_PORT`, so the boot preload maps one to the other (scripts/boot-log.ts). Digits only: `3e3` and
// `0x10` coerce to numbers a reader did not write, and the port is the one setting where a silent
// reinterpretation sends a request to a different place.
const appPort = z.preprocess(
	blankAsAbsent,
	z
		.string()
		.regex(/^\d+$/, 'must be a port number, digits only')
		.default('3000')
		.transform(Number)
		.refine((port) => port >= 1 && port <= 65535, 'must be a port between 1 and 65535')
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

/**
 * The subset the boot preload validates: the runtime environment, and the port the server binds.
 *
 * Kept out of `envSchema` because the two entry points fail differently on purpose. A missing gateway
 * target is a state the panel reports per request rather than a boot failure, so the preload must not
 * pull that requirement forward; a port that cannot be parsed is not bootable at all, so it stops the
 * process before the server module loads. The runtime fields are declared once here, so the two schemas
 * cannot drift into two definitions of a valid port.
 */
const runtimeSchema = z.object({
	APP_ENV: appEnv,
	APP_PORT: appPort
});

export type PanelRuntime = z.infer<typeof runtimeSchema>;

export function parseRuntime(source: Record<string, string | undefined>): PanelRuntime {
	// Same selection-before-parse as parseEnv: the process environment holds every variable, not just
	// ours, and only the declared keys are ours to judge. An unknown PANEL_ variable is parseEnv's
	// business, not the preload's, so a typo still fails the first request rather than the boot.
	const selected: Record<string, string | undefined> = {};
	for (const key of Object.keys(runtimeSchema.shape)) selected[key] = source[key];

	const parsed = runtimeSchema.safeParse(selected);

	if (!parsed.success) {
		const first = parsed.error.issues[0];
		throw new EnvError(first.path.join('.') || 'env', first.message);
	}

	return parsed.data;
}
