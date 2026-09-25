<script lang="ts">
	// The connectivity test for one custom provider node (docs/SPEC-API/001-SPEC-API.md §7.4).
	//
	// A node carries no credential of its own, so the field is optional and the client tells the gateway
	// the difference: an empty field sends no body at all, which is what lets a node whose upstream needs
	// no credential be tested honestly rather than with an empty bearer token.
	//
	// The outcome is rendered rather than thrown. A refused credential answers 200 with a fail state, and
	// the result is stored nowhere: a node has no `test_status` column, so this is an answer to a question
	// and not a state of the object.
	import { testProviderNode } from '$lib/api/provider-nodes';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { nodeTestStateLabel, type ProviderNodeProbe } from '$lib/schemas/provider-node';
	import { formatTimestamp } from '$lib/utils/time';

	const TestIcon = ROW_ACTION_ICONS.test.icon;

	let { providerId }: { providerId: string } = $props();

	let credential = $state('');
	let testing = $state(false);
	let result = $state<ProviderNodeProbe | null>(null);
	let error = $state<string | null>(null);

	async function run(): Promise<void> {
		testing = true;
		error = null;

		const answer = await testProviderNode(providerId, credential);
		testing = false;

		if (!answer.ok) {
			error = answer.error.message;
			result = null;
			return;
		}

		result = answer.data;
	}
</script>

<div class="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3">
	<p class="text-sm">
		The gateway calls the node's base URL and reports whether it answered. A node has no stored
		credential, so the field below is optional: leave it empty to test an upstream that needs none.
	</p>

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Credential (optional)</span>
			<input
				type="password"
				bind:value={credential}
				class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-50"
			disabled={testing}
			onclick={run}
		>
			<TestIcon class="size-4" aria-hidden="true" />
			{testing ? 'Testing' : 'Test the endpoint'}
		</button>
	</div>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]" role="alert">
			The test did not run. {error}
		</p>
	{/if}

	{#if result}
		<p class="text-sm" role="status">
			<span class="font-medium">{nodeTestStateLabel(result.state)}</span>
			in {result.latency_ms} ms{#if result.message}: {result.message}{/if}{#if result.checked_at},
				checked {formatTimestamp(result.checked_at)}{/if}.
		</p>
	{/if}
</div>
