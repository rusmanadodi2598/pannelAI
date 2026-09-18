<script lang="ts">
	// Create one upstream endpoint (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// The provider is fixed rather than chosen here. §6.3 has the Providers screen link to this form with the
	// provider already filled in, and offering a picker would mean loading the registry into a select, which
	// §6.3's pagination discipline rules out. The first credential is optional: an endpoint may be created
	// empty and given keys from the detail drawer, which is the same code path either way.
	import { createEndpoint } from '$lib/api/endpoints';
	import { AUTH_TYPES, AUTH_TYPE_LABELS, schemaCreateEndpointForm } from '$lib/schemas/endpoint';

	let { providerId, oncreated }: { providerId: string; oncreated: () => void } = $props();

	let label = $state('');
	let authType = $state<string>('api_key');
	let priority = $state('');
	let keyValue = $state('');
	let saving = $state(false);
	let error = $state<string | null>(null);

	async function submit(): Promise<void> {
		const parsed = schemaCreateEndpointForm.safeParse({
			provider_id: providerId,
			label,
			auth_type: authType,
			priority: priority === '' ? undefined : priority,
			key_value: keyValue
		});

		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'That endpoint cannot be created.';
			return;
		}

		saving = true;
		const result = await createEndpoint(parsed.data);
		saving = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		label = '';
		priority = '';
		keyValue = '';
		oncreated();
	}
</script>

<div class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3">
	<span class="text-sm font-medium">Add an endpoint for {providerId}</span>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]">{error}</p>
	{/if}

	<div class="flex flex-wrap items-end gap-3 text-sm">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Label</span>
			<input
				bind:value={label}
				placeholder="Primary"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Auth type</span>
			<select
				bind:value={authType}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			>
				{#each AUTH_TYPES as type (type)}
					<option value={type}>{AUTH_TYPE_LABELS[type] ?? type}</option>
				{/each}
			</select>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Priority (optional)</span>
			<input
				bind:value={priority}
				inputmode="numeric"
				placeholder="1"
				class="min-h-11 w-28 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 tabular-nums"
			/>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">First key (optional)</span>
			<input
				bind:value={keyValue}
				type="password"
				autocomplete="off"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>

		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={saving}
			onclick={submit}>Add endpoint</button
		>
	</div>
</div>
