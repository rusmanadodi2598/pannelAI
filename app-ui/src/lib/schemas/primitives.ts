// Reusable field primitives for the panel.
//
// One primitive per field class from docs/SPEC-UI/001-SPEC-UI.md §7.2, so a label or a proxy port
// is validated and sanitized the same way on every screen. Messages live here with the check that
// produces them (SPEC-UI §7.1.8), so a field cannot report two different things in two places.

import { z } from 'zod';
import {
	collapseSpaces,
	hasAngleBrackets,
	normalizeHost,
	normalizeLabelInput,
	normalizeNoProxyList,
	stripKeyWhitespace,
	stripTrailingSlash
} from './sanitize';

export const MAX_LABEL = 120;
export const MAX_SEARCH_TEXT = 200;

export const label = z
	.string()
	.transform(normalizeLabelInput)
	.refine((value) => value.length > 0, { message: 'A name is required.' })
	.refine((value) => value.length <= MAX_LABEL, {
		message: `Use ${MAX_LABEL} characters or fewer.`
	})
	.refine((value) => !hasAngleBrackets(value), {
		message: 'Angle brackets are not allowed in names.'
	});

export const searchText = z
	.string()
	.transform((value) => collapseSpaces(value))
	.refine((value) => value.length <= MAX_SEARCH_TEXT, {
		message: `Use ${MAX_SEARCH_TEXT} characters or fewer.`
	});

export const gatewayKeyInput = z
	.string()
	.transform(stripKeyWhitespace)
	.refine((value) => value.startsWith('sk-'), { message: 'A gateway key starts with sk-.' })
	.refine((value) => value.length >= 20, { message: 'That key looks too short.' });

// Bounded and whitespace-trimmed only. The value is write-only, so nothing downstream may echo it.
export const secretValue = z
	.string()
	.transform((value) => value.trim())
	.refine((value) => value.length >= 8, { message: 'That value looks too short.' })
	.refine((value) => value.length <= 4096, { message: 'Use 4096 characters or fewer.' });

export const proxyHost = z
	.string()
	.transform(normalizeHost)
	.refine((value) => value.length > 0, { message: 'A host is required.' })
	.refine((value) => value.length <= 253, { message: 'Use 253 characters or fewer.' })
	.refine((value) => !/[/\s:]/.test(value), {
		message: 'Enter a host only, with no scheme, path, or port.'
	});

export const proxyPort = z.coerce
	.number()
	.int({ message: 'Use a whole port number.' })
	.min(1, { message: 'Ports start at 1.' })
	.max(65535, { message: 'Ports end at 65535.' });

export const absoluteUrl = z
	.string()
	.transform((value) => stripTrailingSlash(collapseSpaces(value)))
	.refine((value) => value.length > 0, { message: 'A URL is required.' })
	.refine((value) => /^https?:\/\//.test(value), { message: 'Use an http or https URL.' })
	.refine(
		(value) => {
			try {
				new URL(value);
				return true;
			} catch {
				return false;
			}
		},
		{ message: 'That URL cannot be parsed.' }
	);

// Optional URL: the settings surface allows an empty string to mean "not set".
export const optionalAbsoluteUrl = z.union([z.literal(''), absoluteUrl]);

// `outbound_no_proxy` is a comma-separated string on the wire (SPEC-API §7.14, Go
// `domain.NetworkSettings.OutboundNoProxy string`), so the panel keeps it a string end to end. The
// sanitizer normalizes the list (trim, lowercase, dedupe, drop empty) and this schema rejoins it, so a
// typed value and a stored value both round-trip without changing shape.
export const noProxyList = z
	.string()
	.transform((value) => normalizeNoProxyList(value).join(','))
	.refine((hosts) => hosts.split(',').every((host) => host.length <= 253), {
		message: 'Each host must be 253 characters or fewer.'
	});

export const rfc3339Timestamp = z
	.string()
	.refine((value) => !Number.isNaN(Date.parse(value)), { message: 'Invalid timestamp.' });

// A list the API may send as null. Go marshals a nil slice as `null` rather than `[]`, and several response
// shapes carry a list without `omitempty`, so an empty collection arrives as null. It is normalized to an
// empty array here so no caller has to handle both spellings of "none", and it is the one place that rule
// lives: `stringList` below is the string case of the same transform.
export function nullableList<T extends z.ZodType>(item: T) {
	return z
		.array(item)
		.nullish()
		.transform((value) => value ?? []);
}

export const stringList = nullableList(z.string());

// The pagination block SPEC-API §4 returns beside every list. It lives here rather than in each resource
// file because a second definition would be a second answer to "what does the panel expect from `meta`".
export const pageMeta = z.object({
	page: z.number().int(),
	per_page: z.number().int(),
	total: z.number().int()
});

export const nullableTimestamp = rfc3339Timestamp.nullable();

// The endpoint shapes mark their timestamps `omitempty` on the wire (Go omits a nil pointer), so an
// absent field and an explicit null both mean "never happened". `nullish` accepts both without needing a
// separate union at every field, and it is the reason this exists next to `nullableTimestamp`.
export const optionalTimestamp = rfc3339Timestamp.nullish();

// The API bounds a priority at 1 to 10000 (SPEC-API §7.5, `min=1,max=10000`). Kept as a primitive so the
// panel and the validator cannot disagree about the range.
export const priority = z.coerce
	.number()
	.int({ message: 'Use a whole number for priority.' })
	.min(1, { message: 'Priority starts at 1.' })
	.max(10000, { message: 'Priority ends at 10000.' });

export const costString = z
	.string()
	.refine((value) => /^\d+(\.\d+)?$/.test(value), { message: 'Invalid cost value.' });

export const tokenCount = z
	.number()
	.int({ message: 'Token counts are whole numbers.' })
	.min(0, { message: 'Token counts cannot be negative.' });

export const portNumber = z.number().int().min(1).max(65535);

export const pageNumber = z.coerce.number().int().min(1).default(1);

// The API caps per_page at 100 (SPEC-API §4), so a hand-edited URL cannot widen the page.
export const perPage = z.coerce.number().int().min(1).max(100).default(25);

export const password = z
	.string()
	.refine((value) => value.length >= 1, { message: 'A password is required.' })
	.refine((value) => value.length <= 200, { message: 'Use 200 characters or fewer.' });

// How a recorded request ended. One field class, two resources: a usage record and a request log both
// carry it, and both come from the same closed set in app-serv (`UsageStatus`, which the log aggregate
// reuses), so it is declared once here rather than twice with a drift risk between them. SPEC-UI §7.4.3
// makes an unknown member an error rather than a value rendered verbatim.
export const REQUEST_STATUSES = ['success', 'error'] as const;
export type RequestStatus = (typeof REQUEST_STATUSES)[number];

export const REQUEST_STATUS_LABELS: Record<RequestStatus, string> = {
	success: 'Success',
	error: 'Error'
};

export const schemaRequestStatus = z.enum(REQUEST_STATUSES);

// Endpoints that answer 204 carry no body, and the client still parses through a schema. Loose so an
// API that starts returning a body does not fail the panel.
export const emptyResponse = z.looseObject({});
export type EmptyResponse = z.infer<typeof emptyResponse>;

export function prefixedId(prefix: string) {
	return z
		.string()
		.refine((value) => value.startsWith(prefix), { message: `Expected an ${prefix} identifier.` });
}

export const gatewayKeyId = prefixedId('gky_');

// The upstream identifiers the API mints (SPEC-API §7.5). Listed here beside the gateway key prefix so
// the panel has one place that knows what an identifier looks like.
export const endpointId = prefixedId('ep_');
export const upstreamKeyId = prefixedId('uky_');
