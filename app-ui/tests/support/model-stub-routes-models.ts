// The model, combo and catalog branches of the model stub (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The merged catalog excludes a disabled model and a disabled-set write replaces the whole set, which is
// why both read the stub's own state here rather than echoing the request back. The final refusal lives
// in this module because this family is the last one the dispatcher asks.

import type { StubContext } from './model-stub-context';
import type { StubModel } from './model-stub-types';
import { customRow } from './model-stub-fixtures';

export function handleModelRoutes({
	stub,
	method,
	body,
	parsed,
	json,
	refusal,
	key,
	blocked,
	catalogWithCustom
}: StubContext): Response {
	if (parsed.pathname.endsWith('/models/disabled')) {
		if (method === 'PUT') {
			const models = (body.models ?? []) as StubModel[];
			stub.disabledWrites.push(models);

			if (stub.writeStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The set could not be stored.', stub.writeStatus);
			}

			const available = new Set(catalogWithCustom().map(key));
			for (const ref of models) {
				if (!available.has(key(ref))) {
					return refusal('VALIDATION_ERROR', `unknown model: ${key(ref)}`, 400);
				}
			}

			stub.disabled = models.map((ref) => ({ ...ref }));
			return json({ data: stub.disabled });
		}

		stub.reads.push('disabled');
		if (stub.disabledReadStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The set could not be read.', stub.disabledReadStatus);
		}
		return json({ data: stub.disabled });
	}

	if (parsed.pathname.endsWith('/models/custom')) {
		if (method === 'POST') {
			stub.customCreates.push(body);

			if (stub.writeStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The row could not be stored.', stub.writeStatus);
			}

			const providerId = String(body.provider_id ?? '');
			if (!stub.providers.includes(providerId)) {
				return refusal('VALIDATION_ERROR', `unknown provider_id: ${providerId}`, 400);
			}
			if (catalogWithCustom().some((row) => key(row) === `${providerId}/${body.model_id}`)) {
				return refusal('CONFLICT', 'model already exists: models_custom_pair', 409);
			}

			const row = customRow({
				id: `mdl_${String(stub.custom.length + 1).padStart(2, '0')}`,
				provider_id: providerId,
				model_id: body.model_id,
				display_name: body.display_name,
				capabilities: body.capabilities ?? []
			});
			stub.custom = [row, ...stub.custom];
			return json(row, 201);
		}

		stub.reads.push('custom');
		if (stub.readStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The rows could not be read.', stub.readStatus);
		}
		return json({ data: stub.custom });
	}

	const deleteMatch = /\/models\/custom\/([^/?]+)$/.exec(parsed.pathname);
	if (method === 'DELETE' && deleteMatch) {
		const id = decodeURIComponent(deleteMatch[1]);
		stub.customDeletes.push(id);

		if (stub.writeStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The row could not be removed.', stub.writeStatus);
		}

		const row = stub.custom.find((entry) => entry.id === id);
		if (!row) return refusal('NOT_FOUND', 'model is not in the catalog', 404);

		stub.custom = stub.custom.filter((entry) => entry.id !== id);
		return new Response(null, { status: 204 });
	}

	if (method === 'GET' && parsed.pathname.endsWith('/combos')) {
		stub.reads.push('combos');

		const page = Number(parsed.searchParams.get('page') ?? 1);
		const perPage = Number(parsed.searchParams.get('per_page') ?? 25);
		const start = (page - 1) * perPage;
		return json({
			data: stub.combos.slice(start, start + perPage),
			meta: {
				page,
				per_page: perPage,
				total: stub.combosTotal ?? stub.combos.length
			}
		});
	}

	// The probe route answers from the stored combo, not from the request: the id it was asked for is
	// the combo whose name and strategy come back, so a panel that rendered its own row's values would
	// pass against a stub that echoed the body back.
	const comboTestMatch = /\/combos\/([^/?]+)\/test$/.exec(parsed.pathname);
	if (method === 'POST' && comboTestMatch) {
		const id = decodeURIComponent(comboTestMatch[1]);
		stub.comboTests.push(id);

		if (stub.comboTestStatus !== 200) {
			return refusal('NOT_FOUND', 'combo not found', stub.comboTestStatus);
		}

		const combo = stub.combos.find((entry) => entry.id === id);
		if (!combo) return refusal('NOT_FOUND', 'combo not found', 404);

		return json({
			combo_id: combo.id,
			combo: combo.name,
			strategy: combo.strategy,
			results: stub.comboTestResults
		});
	}

	// The model probe. The single-model route answers from `modelTestRows`, keyed by the model id the
	// panel named, so a row that failed is reported for the model it belongs to rather than echoed back
	// from the request. The sweep answers for the provider's whole list and reports how much of it ran.
	const modelTestMatch = /\/providers\/([^/?]+)\/models\/test$/.exec(parsed.pathname);
	if (method === 'POST' && modelTestMatch) {
		const providerId = decodeURIComponent(modelTestMatch[1]);
		const modelId = String(body.model_id ?? '');
		stub.modelTests.push(`${providerId}/${modelId}`);

		if (stub.modelTestStatus !== 200) {
			return refusal('VALIDATION_ERROR', 'The model test was refused.', stub.modelTestStatus);
		}

		const row = stub.modelTestRows.find((entry) => entry.model_id === modelId);
		if (row) return json(row);
		return json({ model_id: modelId, name: modelId, ok: true, latency_ms: 40 });
	}

	const sweepMatch = /\/providers\/([^/?]+)\/test-models$/.exec(parsed.pathname);
	if (method === 'POST' && sweepMatch) {
		const providerId = decodeURIComponent(sweepMatch[1]);
		stub.modelSweeps.push({ provider: providerId, limit: Number(body.limit ?? 0) });

		if (stub.modelTestStatus !== 200) {
			return refusal(
				'VALIDATION_ERROR',
				'This provider offers no chat model to test.',
				stub.modelTestStatus
			);
		}

		return json({
			provider_id: providerId,
			source: 'registry',
			tested: stub.modelTestRows.length,
			total: stub.modelTestTotal ?? stub.modelTestRows.length,
			stopped: stub.modelTestStopped ?? '',
			results: stub.modelTestRows
		});
	}

	if (parsed.pathname.endsWith('/models/catalog')) {
		stub.reads.push(`catalog:${parsed.searchParams.get('provider_id') ?? ''}`);

		if (stub.readStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The catalog could not be read.', stub.readStatus);
		}

		const providerId = parsed.searchParams.get('provider_id') ?? '';
		const capability = parsed.searchParams.get('capability') ?? '';
		const query = (parsed.searchParams.get('q') ?? '').toLowerCase();
		const hidden = blocked();

		// The merged view, with disabled rows removed: the server hides a disabled pair from both its
		// halves, so a disabled custom row is hidden too.
		const answer = catalogWithCustom().filter((row) => {
			if (hidden.has(key(row))) return false;
			if (providerId !== '' && row.provider_id !== providerId) return false;
			if (capability !== '' && !((row.capabilities ?? []) as string[]).includes(capability)) {
				return false;
			}
			if (query !== '' && !`${row.model_id} ${row.display_name}`.toLowerCase().includes(query)) {
				return false;
			}
			return true;
		});

		return json({ data: answer });
	}

	return refusal('NOT_FOUND', 'No route matches that request.', 404);
}
