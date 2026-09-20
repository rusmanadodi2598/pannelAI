<script lang="ts">
	// The combo test readout (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// The test is a modal because its answer is a list of several references and it is worth reading beside
	// the row that asked for it, not inside a table cell. It does not run on open: the click spends one
	// account per reference, one at a time, and §7.7 writes no usage row for that spend, so the operator
	// confirms with the sentence in front of them rather than discovering it after.
	//
	// A reference that failed is rendered as a result, not as an error: `ok`, the gateway's own code, and
	// the gateway's own message, attributed to it. The panel does not paraphrase a failure or colour the
	// whole combo by one member's outcome.
	//
	// The panel cannot cancel a sequential server-side loop, so Close stays available while the probes run
	// and the dialog does not pretend to stop them. That is the other reason the spend is stated first.
	import Modal from '$lib/components/Modal.svelte';
	import { testCombo } from '$lib/api/combos';
	import {
		comboProbeIdentity,
		comboProbeRoleLabel,
		comboProbeSummary,
		type ComboTest
	} from '$lib/schemas/combo-test';
	import { comboStrategyLabel, type Combo } from '$lib/schemas/combo';

	let { combo, onclose }: { combo: Combo | null; onclose: () => void } = $props();

	let running = $state(false);
	let answer = $state<ComboTest | null>(null);
	let error = $state<string | null>(null);
	let runToken = 0;

	// A closed dialog takes its report with it, and the next open starts clean: the run that was left behind
	// spent its accounts already, so showing a stale report beside a fresh button would misstate both. A
	// completed request from the closed dialog is ignored by the same token, because the API cannot cancel it.
	$effect(() => {
		if (combo === null) {
			runToken += 1;
			running = false;
			answer = null;
			error = null;
		}
	});

	function close(): void {
		runToken += 1;
		onclose();
	}

	async function run(): Promise<void> {
		if (combo === null) return;

		const token = ++runToken;
		running = true;
		error = null;
		answer = null;
		const result = await testCombo(combo.id);
		if (token !== runToken) return;
		running = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		answer = result.data;
	}
</script>

<Modal title="Test this combo" open={combo !== null} onclose={close}>
	{#if combo}
		<p>
			Test <span class="font-medium">{combo.name}</span> ({comboStrategyLabel(combo.strategy)})?
			Each reference is probed in turn with a one-token request, so the test spends one account at a
			time. The probes are not recorded in usage.
		</p>
	{/if}

	{#if running}
		<p class="mt-3 text-[var(--color-text-muted)]" role="status">
			Probing every reference, one at a time. This waits on the upstreams themselves.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			The combo could not be tested. {error} If the combo was deleted, reload the list to see the current
			set.
		</p>
	{/if}

	{#if answer}
		<p class="mt-3" role="status">{comboProbeSummary(answer.results)}</p>

		{#if answer.results.length > 0}
			<ul class="mt-3 flex flex-col gap-2">
				{#each answer.results as result (result.ref + result.role)}
					<li class="rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 py-2">
						<div class="flex flex-wrap items-center gap-2">
							<span class="font-medium">{result.ref}</span>
							<span class="text-xs text-[var(--color-text-muted)]"
								>{comboProbeRoleLabel(result.role)}</span
							>
							<span class="ms-auto tabular-nums" class:text-[var(--color-danger)]={!result.ok}>
								{result.ok ? 'Answered' : 'Failed'} in {result.latency_ms} ms
							</span>
						</div>
						<p class="text-[var(--color-text-muted)]">
							{comboProbeIdentity(result)}
							{#if result.endpoint_id !== ''}
								via {result.endpoint_id}
							{/if}
						</p>
						{#if !result.ok && (result.error_code !== '' || result.error !== '')}
							<p class="text-[var(--color-danger)]">
								{result.error_code}{result.error_code !== '' && result.error !== ''
									? ': '
									: ''}{result.error}
							</p>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	{/if}

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={close}>Close</button>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={running}
			onclick={() => void run()}
		>
			{running ? 'Testing' : answer ? 'Test again' : 'Test the chain'}
		</button>
	{/snippet}
</Modal>
