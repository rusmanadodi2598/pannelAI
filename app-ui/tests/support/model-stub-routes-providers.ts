// The provider, node and settings branches of the model stub (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The settings document comes first because the Connections section, the proxy card and the reasoning
// picker all read it; the node read is matched before the provider read because both end in a segment.

import type { StubContext } from './model-stub-context';
import type { StubModel } from './model-stub-types';
import { deriveAuthModes, providerDetailRow } from './model-stub-fixtures';

export function handleProviderRoutes({
	stub,
	method,
	body,
	parsed,
	json,
	refusal
}: StubContext): Response | null {
	if (parsed.pathname.endsWith('/settings')) {
		if (method === 'PATCH') {
			stub.settingsPatches.push(body);

			if (stub.settingsWriteStatus !== 200) {
				return refusal(
					'VALIDATION_ERROR',
					'The gateway refused this value.',
					stub.settingsWriteStatus
				);
			}

			const merged = { ...stub.settings };
			for (const [group, value] of Object.entries(body)) {
				merged[group] = { ...(stub.settings[group] as StubModel), ...(value as StubModel) };
			}
			stub.settings = merged;
		}

		if (stub.settingsReadStatus !== 200) {
			return refusal(
				'INTERNAL_ERROR',
				'The settings store is unreachable.',
				stub.settingsReadStatus
			);
		}

		return json(stub.settings);
	}

	// The proxy pool list, which the proxy card reads for its pool select. The card renders on this
	// screen too, so the route is served here rather than by a second stub that would fight over
	// `fetch`.
	if (method === 'GET' && parsed.pathname.endsWith('/proxies')) {
		if (stub.proxyReadStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The pool could not be read.', stub.proxyReadStatus);
		}
		return json({ data: stub.proxies });
	}

	// The node's own read, which the details card makes when the screen is a custom node's (§7.4). It is
	// matched before the provider read below because both end in a path segment.
	const nodeMatch = /\/provider-nodes\/([^/?]+)$/.exec(parsed.pathname);
	if (method === 'GET' && nodeMatch) {
		if (stub.providerNode === null) {
			return refusal('NOT_FOUND', 'That provider no longer exists.', 404);
		}
		return json(stub.providerNode);
	}

	// The page's own load, and the endpoint list its last section reads. Both answer the shape the
	// panel's schemas expect, so a test can render the whole screen and assert on one section.
	//
	// The provider's own model list is checked first, because its path is the provider path plus a
	// segment: a node's import reads it, and the answer is the rows plus, when the list is a fallback
	// rather than a fresh answer, the warning that says so.
	const providerModelsMatch = /\/providers\/([^/?]+)\/models$/.exec(parsed.pathname);
	if (method === 'GET' && providerModelsMatch) {
		stub.providerModelsReads.push(decodeURIComponent(providerModelsMatch[1]));

		if (stub.providerModelsReadStatus !== 200) {
			return refusal(
				'INTERNAL_ERROR',
				'The provider models could not be read.',
				stub.providerModelsReadStatus
			);
		}

		return json({
			data: stub.providerModels,
			...(stub.providerModelsWarning === null ? {} : { warning: stub.providerModelsWarning })
		});
	}

	// The provider list, which the picker reads for its active set and its group names. It is served
	// paginated the way the route is, so a test can hold the loader's page walk rather than a
	// one-page answer that would pass for a loader which never asked for a second page.
	if (method === 'GET' && parsed.pathname.endsWith('/providers')) {
		const page = Number(parsed.searchParams.get('page') ?? 1);
		const perPage = Number(parsed.searchParams.get('per_page') ?? 25);
		const start = (page - 1) * perPage;
		return json({
			data: stub.providerRows.slice(start, start + perPage),
			meta: { page, per_page: perPage, total: stub.providerRows.length }
		});
	}

	const providerMatch = /\/providers\/([^/?]+)$/.exec(parsed.pathname);
	if (method === 'GET' && providerMatch) {
		return json(
			providerDetailRow({
				id: decodeURIComponent(providerMatch[1]),
				auth_type: stub.authType,
				has_oauth: stub.hasOAuth,
				auth_modes:
					stub.authModes.length > 0
						? stub.authModes
						: deriveAuthModes(stub.authType, stub.hasOAuth),
				// Absent when no hint is declared, which is the shape Go's omitempty answers and the
				// schema's nullableText normalizes to null.
				...(stub.authHint === null ? {} : { auth_hint: stub.authHint }),
				// Absent when the test declared no levels, which is the shape a provider whose models
				// do not reason answers (the DTO marks the field `omitempty`).
				...(stub.thinkingLevels === null ? {} : { thinking_levels: stub.thinkingLevels })
			})
		);
	}

	return null;
}
