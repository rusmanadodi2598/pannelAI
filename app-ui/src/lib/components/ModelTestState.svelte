<script lang="ts">
	// One model's probe answer, rendered where the model is listed (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Two tables show models a probe can answer for: the provider's catalog and the rows an operator
	// declared for a compatible node. The verdict belongs to the model, not to the table, so the state
	// lives here once rather than twice, where the two copies could disagree about what a failure reads as.
	//
	// `unreachable` is kept distinct from a model that refused: the first says the panel never got an
	// answer, the second says the model gave one. Rendering them alike would blame a model for a request
	// that never reached it (R-36).
	import { LoaderCircle } from '@lucide/svelte';
	import { modelTestLine } from '$lib/schemas/model-test';
	import type { ModelTestRow } from '$lib/stores/model-test.svelte';

	let { probe, routable = true }: { probe?: ModelTestRow; routable?: boolean } = $props();
</script>

{#if probe === undefined}
	<span class="text-[var(--color-text-muted)]">
		{routable ? 'Not tested' : 'Not a chat model'}
	</span>
{:else if probe.phase === 'running'}
	<span class="inline-flex items-center gap-2" role="status">
		<LoaderCircle class="size-4 animate-spin" aria-hidden="true" />
		Testing
	</span>
{:else if probe.phase === 'unreachable'}
	<span class="text-[var(--color-danger)]" role="alert">
		The panel could not ask
		<br />
		<span class="break-words text-xs">{probe.message}</span>
	</span>
{:else}
	<span
		class={probe.result.ok ? '' : 'text-[var(--color-danger)]'}
		role={probe.result.ok ? 'status' : 'alert'}
	>
		{modelTestLine(probe.result)}
		{#if probe.result.error_code !== ''}
			<br />
			<span class="break-words text-xs">
				{probe.result.error_code}: {probe.result.error}
			</span>
		{/if}
	</span>
{/if}
