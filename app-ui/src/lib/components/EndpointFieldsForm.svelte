<script lang="ts">
	// The endpoint's own fields inside the detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Split out because the drawer also holds the routing answer, the keys table, and the add-key form, and
	// one file holding all four goes past the project line limit. This owns the three editable fields and
	// their save, which is a self-contained unit: it validates through the shared schema, reports its own
	// outcome, and hands the updated endpoint back so the drawer and the list can both reflect it.
	import { updateEndpoint } from '$lib/api/endpoints';
	import {
		ENDPOINT_STATUS_ACTIVE,
		ENDPOINT_STATUS_DISABLED,
		schemaUpdateEndpointForm,
		type Endpoint
	} from '$lib/schemas/endpoint';

	let { endpoint, onsaved }: { endpoint: Endpoint; onsaved: (updated: Endpoint) => void } =
		$props();

	// The draft fields start empty and are filled from the endpoint, then refilled whenever a different one
	// is handed in. Seeding them at declaration would capture only the first endpoint, so switching rows
	// would leave the previous row's values in the inputs.
	let label = $state('');
	let priority = $state('');
	let saving = $state(false);
	let notice = $state<string | null>(null);

	const active = $derived(endpoint.status === ENDPOINT_STATUS_ACTIVE);

	$effect(() => {
		label = endpoint.label;
		priority = String(endpoint.priority);
	});

	async function save(): Promise<void> {
		const parsed = schemaUpdateEndpointForm.safeParse({
			label,
			priority: priority === '' ? undefined : priority
		});

		if (!parsed.success) {
			notice = parsed.error.issues[0]?.message ?? 'That change cannot be saved.';
			return;
		}

		saving = true;
		const result = await updateEndpoint(endpoint.id, parsed.data);
		saving = false;

		if (!result.ok) {
			notice = result.error.message;
			return;
		}

		// A priority change reorders siblings, so the caller refreshes the list rather than assuming (§6.2).
		notice = 'Saved. Priority decides the order, so the list behind this drawer refreshes too.';
		onsaved(result.data);
	}

	async function toggleStatus(): Promise<void> {
		const result = await updateEndpoint(endpoint.id, {
			status: active ? ENDPOINT_STATUS_DISABLED : ENDPOINT_STATUS_ACTIVE
		});

		if (!result.ok) {
			notice = result.error.message;
			return;
		}

		onsaved(result.data);
	}
</script>

<div class="flex flex-col gap-3">
	{#if notice}
		<p class="text-[var(--color-text-muted)]">{notice}</p>
	{/if}

	<div class="grid gap-3 sm:grid-cols-3">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Label</span>
			<input
				bind:value={label}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Priority</span>
			<input
				bind:value={priority}
				inputmode="numeric"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 tabular-nums"
			/>
		</label>
		<div class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Status</span>
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2"
				onclick={toggleStatus}
			>
				{active ? 'Active' : endpoint.status}. Change
			</button>
		</div>
	</div>

	<div>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={saving}
			onclick={save}>Save endpoint</button
		>
	</div>
</div>
