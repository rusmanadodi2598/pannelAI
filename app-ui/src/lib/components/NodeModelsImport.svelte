<script lang="ts">
	// The `/models` import on a custom node's models section (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The reference's own control (`providers/[id]/CompatibleModelsSection.js:125-159`): ask the node's
	// upstream what it serves, and declare every model this node does not carry yet. It is its own component
	// because the flow owns three states the list around it does not (importing, the gate, the outcome),
	// and because the models section is at its own size limit.
	//
	// The gate is the reference's too (`:161`, `:190-194`): a node whose connections are all disabled has
	// nothing to ask, so the control is disabled with the sentence that says what to do about it. The read
	// is this component's own, and a read that fails leaves the gate open: the import's own answer is the
	// truth, and a control disabled by a read that never landed would hide a working action.
	import { untrack } from 'svelte';
	import { listEndpoints } from '$lib/api/endpoints';
	import { createCustomModel } from '$lib/api/models';
	import { listProviderModels } from '$lib/api/providers';
	import { customModelBody, type CustomModel } from '$lib/schemas/custom-model';
	import { ENDPOINT_STATUS_DISABLED } from '$lib/schemas/endpoint';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';

	let {
		providerId,
		known,
		onimported
	}: {
		providerId: string;
		/** The model ids this node already carries, so an import never re-declares one. */
		known: string[];
		onimported: (rows: CustomModel[]) => void;
	} = $props();

	// The control keeps its label and gains the glyph beside it; the success notice renders the Check
	// glyph from the same map rather than a tick character typed into the string (PORT 002 D6).
	const ImportIcon = CONTROL_ICONS.import.icon;
	const CheckIcon = ROW_ACTION_ICONS.save.icon;

	let importing = $state(false);
	let blocked = $state(false);
	let notice = $state<string | null>(null);
	// Whether the last notice was a full success, which is what the Check glyph marks.
	let noticeOk = $state(false);
	let error = $state<string | null>(null);
	/** The gateway's own sentence for a list that is not the upstream's, when it sends one. */
	let warning = $state<string | null>(null);

	$effect(() => {
		untrack(() => void readGate());
	});

	async function readGate(): Promise<void> {
		const result = await listEndpoints({ provider_id: providerId, page: 1, per_page: 100 });
		blocked = result.ok && !result.data.data.some((row) => row.status !== ENDPOINT_STATUS_DISABLED);
	}

	async function run(): Promise<void> {
		if (importing) return;

		importing = true;
		notice = null;
		error = null;
		warning = null;

		const result = await listProviderModels(providerId);
		if (!result.ok) {
			importing = false;
			error = result.error.message;
			return;
		}
		warning = result.data.warning ?? null;

		const seen = new Set(known);
		const fresh = result.data.data.filter((row) => !seen.has(row.id));

		// The two answers the reference reports (`:139-153`): nothing came back, or everything that came
		// back is already declared. Neither is an error, so neither reads as one.
		if (result.data.data.length === 0) {
			importing = false;
			notice = 'No models returned from /models.';
			return;
		}
		if (fresh.length === 0) {
			importing = false;
			notice = 'No new models were added.';
			return;
		}

		// One write per model, in the order the upstream answered. A refusal stops the run rather than
		// being skipped, because the next write would meet the same rule; what landed before it is kept and
		// reported, so the operator sees the state the gateway is actually in.
		const added: CustomModel[] = [];
		let refusal: string | null = null;
		for (const row of fresh) {
			const created = await createCustomModel(
				customModelBody(providerId, {
					model_id: row.id,
					display_name: row.name ?? '',
					capabilities: ''
				})
			);

			if (!created.ok) {
				refusal = `${row.id}: ${created.error.message}`;
				break;
			}
			added.push(created.data);
		}

		importing = false;
		if (added.length > 0) onimported(added);

		noticeOk = refusal === null && added.length > 0;
		if (refusal === null) {
			notice = `${added.length} imported.`;
		} else if (added.length === 0) {
			notice = `Nothing was imported. The gateway refused ${refusal}`;
		} else {
			notice = `${added.length} imported, then the gateway refused ${refusal}`;
		}
	}
</script>

<div class="flex flex-wrap items-center gap-3">
	<button
		type="button"
		class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 disabled:opacity-50"
		disabled={importing || blocked}
		onclick={() => void run()}
	>
		<ImportIcon class="size-4" aria-hidden="true" />
		{importing ? 'Importing' : 'Import from /models'}
	</button>

	{#if blocked}
		<span class="text-sm text-[var(--color-text-muted)]">
			Add a connection to enable importing models.
		</span>
	{/if}

	{#if warning}
		<span class="text-sm text-[var(--color-text-muted)]">{warning}</span>
	{/if}

	{#if notice}
		<span class="inline-flex items-center gap-1.5 text-sm" role="status">
			{#if noticeOk}
				<CheckIcon class="size-4" aria-hidden="true" />
			{/if}
			{notice}
		</span>
	{/if}

	{#if error}
		<span class="text-sm text-[var(--color-danger)]" role="alert">
			The models were not imported. {error}
		</span>
	{/if}
</div>
