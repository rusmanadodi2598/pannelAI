// Per-model test state for one provider's catalog (docs/SPEC-API/001-SPEC-API.md §7.4, draft 017 §4.10).
//
// One owner for two entrances: a row's own test button and the sweep over the first models. Both write the
// same record, so a sweep that answers fills the rows the operator can then re-test one at a time, and a
// row that is being probed reads as running whichever control started it.
//
// The record is keyed by `provider/model`, the string a client would send, because that is the identity the
// route reports results under. It is transient like the node test: the gateway stores no per-model verdict,
// so the panel shows what the last probe said and nothing it made up (R-36).

import { testProviderModel, testProviderModels } from '$lib/api/providers';
import {
	modelTestRowKey,
	modelTestSweepSummary,
	MODEL_TEST_SWEEP_LIMIT,
	type ModelTestResult
} from '$lib/schemas/model-test';

// One row's state. `unreachable` is the case where the panel never got an answer at all (the route refused
// or the network failed), which is a different fact from a model that answered with a refusal.
export type ModelTestRow =
	| { phase: 'running' }
	| { phase: 'probed'; result: ModelTestResult }
	| { phase: 'unreachable'; message: string };

export function createModelTestStore() {
	let rows = $state<Record<string, ModelTestRow>>({});
	let sweeping = $state(false);
	let summary = $state('');
	let error = $state<string | null>(null);

	const running = $derived(Object.values(rows).some((row) => row.phase === 'running') || sweeping);

	// A fresh read of the catalog drops the last probe's answers: the rows may no longer be the same set,
	// and a verdict left beside a model it never named is the lie this file exists to avoid.
	function clear(): void {
		rows = {};
		summary = '';
		error = null;
	}

	function record(providerId: string, modelId: string, row: ModelTestRow): void {
		rows[modelTestRowKey(providerId, modelId)] = row;
	}

	async function testOne(providerId: string, modelId: string): Promise<void> {
		record(providerId, modelId, { phase: 'running' });
		const answer = await testProviderModel(providerId, modelId);

		if (!answer.ok) {
			record(providerId, modelId, { phase: 'unreachable', message: answer.error.message });
			return;
		}
		record(providerId, modelId, { phase: 'probed', result: answer.data });
	}

	// The sweep answers for the models it probed and says how many it skipped, so an operator reading
	// "4 of 41" knows the other 37 are untested rather than healthy.
	async function testSweep(providerId: string): Promise<void> {
		sweeping = true;
		error = null;
		// The previous count is dropped with the sweep that produced it, so a refusal never renders beside
		// a tally of models this answer did not cover.
		summary = '';
		const answer = await testProviderModels(providerId, MODEL_TEST_SWEEP_LIMIT);
		sweeping = false;

		if (!answer.ok) {
			error = answer.error.message;
			return;
		}

		for (const result of answer.data.results) {
			record(answer.data.provider_id, result.model_id, { phase: 'probed', result });
		}
		// The sentence is the schema's, because it already carries the counts, the truncation, and the
		// list's origin; a second phrasing here could only disagree with it.
		summary = modelTestSweepSummary(answer.data);
	}

	return {
		row: (providerId: string, modelId: string): ModelTestRow | undefined =>
			rows[modelTestRowKey(providerId, modelId)],
		testOne,
		testSweep,
		clear,
		get sweeping(): boolean {
			return sweeping;
		},
		get running(): boolean {
			return running;
		},
		get summary(): string {
			return summary;
		},
		get error(): string | null {
			return error;
		}
	};
}

export type ModelTestStore = ReturnType<typeof createModelTestStore>;
