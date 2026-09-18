<script lang="ts">
	// Add one credential to an endpoint (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Its own component because the drawer is already at the project line limit and because the form owns
	// three pieces of state that the drawer has no use for. The value is a password input with autocomplete
	// off, so a browser does not offer to remember a credential it must not store, and the panel never keeps
	// the value after the submit (SPEC-UI §4).
	import { addEndpointKey } from '$lib/api/endpoints';
	import { schemaAddEndpointKeyForm } from '$lib/schemas/endpoint';

	let { endpointId, onadded }: { endpointId: string; onadded: () => void } = $props();

	let value = $state('');
	let label = $state('');
	let adding = $state(false);
	let error = $state<string | null>(null);

	async function submit(): Promise<void> {
		const parsed = schemaAddEndpointKeyForm.safeParse({
			label: label === '' ? undefined : label,
			value
		});

		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'That key cannot be added.';
			return;
		}

		adding = true;
		const result = await addEndpointKey(endpointId, parsed.data);
		adding = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		value = '';
		label = '';
		onadded();
	}
</script>

<div class="flex flex-col gap-2">
	<span class="font-medium">Add a key</span>

	{#if error}
		<p class="text-[var(--color-danger)]">{error}</p>
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Label (optional)</span>
			<input
				bind:value={label}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Key</span>
			<input
				bind:value
				type="password"
				autocomplete="off"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={adding}
			onclick={submit}>Add key</button
		>
	</div>
</div>
