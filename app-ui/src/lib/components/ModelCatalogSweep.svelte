<script lang="ts">
	// The bounded sweep over a provider's models (docs/SPEC-API/001-SPEC-API.md §7.4, draft 017 §4.10).
	//
	// This is the second entrance to the same probe the catalog rows offer, so it writes into the store the
	// table reads: a sweep that answers fills the rows, and an operator can re-test one of them without
	// running the sweep again.
	//
	// The button names how many models one click spends, because every row is a real inference call against
	// the provider account's quota (SPEC-UI §8.11, antislop R-36). The server holds the ceiling and clamps a
	// larger request; this says the default, which is what the request sends.
	import { LoaderCircle } from '@lucide/svelte';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { MODEL_TEST_SWEEP_LIMIT } from '$lib/schemas/model-test';
	import type { ModelTestStore } from '$lib/stores/model-test.svelte';

	let { providerId, tests }: { providerId: string; tests: ModelTestStore } = $props();

	const TestIcon = ROW_ACTION_ICONS.test.icon;
</script>

<div class="flex flex-col items-start gap-2">
	<button
		type="button"
		class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-50"
		disabled={tests.running}
		onclick={() => void tests.testSweep(providerId)}
	>
		{#if tests.sweeping}
			<LoaderCircle class="size-4 animate-spin" aria-hidden="true" />
			Testing the first {MODEL_TEST_SWEEP_LIMIT} models
		{:else}
			<TestIcon class="size-4" aria-hidden="true" />
			Test the first {MODEL_TEST_SWEEP_LIMIT} models
		{/if}
	</button>

	{#if tests.summary}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{tests.summary}</p>
	{/if}

	{#if tests.error}
		<p class="text-sm text-[var(--color-danger)]" role="alert">{tests.error}</p>
	{/if}
</div>
