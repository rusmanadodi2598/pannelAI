<script lang="ts">
	// The Combos tab of /combos (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 1).
	//
	// The tab owns the page of combos, which one is being edited, and the delete confirmation. The table and
	// the editor are their own components, so this file holds the list's states and the two writes.
	//
	// The reference suggestions for the editor come from the two sources the panel can read: the catalog
	// and the combo names on this page. A ref may be either, so both are offered.
	import { untrack } from 'svelte';
	import ComboDeleteDialog from '$lib/components/ComboDeleteDialog.svelte';
	import ComboEditor from '$lib/components/ComboEditor.svelte';
	import ComboTable from '$lib/components/ComboTable.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteCombo, listCombos } from '$lib/api/combos';
	import { listModelCatalog } from '$lib/api/models';
	import type { Combo } from '$lib/schemas/combo';

	const PAGE_SIZE = 25;

	let combos = $state<Combo[]>([]);
	let total = $state(0);
	let pageNumber = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let editing = $state<Combo | null>(null);
	let creating = $state(false);
	let pendingDelete = $state<Combo | null>(null);
	let deleting = $state<string | null>(null);
	let deleteError = $state<string | null>(null);
	// Whether the last delete failure was the API's CONFLICT, which is the only answer that means an alias
	// still references the combo.
	let deleteConflict = $state(false);

	let catalogRefs = $state<string[]>([]);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const showEditor = $derived(creating || editing !== null);

	// The editor's suggestions, deduplicated because a combo name may also be a catalog id and offering the
	// same string twice is noise in a picker.
	const suggestions = $derived(
		[...new Set([...catalogRefs, ...combos.map((combo) => combo.name)])].sort()
	);

	$effect(() => {
		untrack(() => {
			void load();
			void loadSuggestions();
		});
	});

	async function load(): Promise<void> {
		loading = true;
		const result = await listCombos({ page: pageNumber, per_page: PAGE_SIZE });
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		combos = result.data.data;
		total = result.data.meta.total;
	}

	// The suggestions are a convenience, so a failure here is silent: the editor still works with a typed
	// reference, and a banner about a picker would be noise beside the list's own error.
	async function loadSuggestions(): Promise<void> {
		const catalog = await listModelCatalog({});
		if (catalog.ok) catalogRefs = catalog.data.data.map((model) => model.id);
	}

	// Both halves of a delete failure are cleared together, so a stale conflict flag can never colour the
	// copy of a later failure.
	function clearDeleteError(): void {
		deleteError = null;
		deleteConflict = false;
	}

	function closeEditor(): void {
		creating = false;
		editing = null;
	}

	async function confirmDelete(): Promise<void> {
		const target = pendingDelete;
		if (target === null) return;

		deleting = target.id;
		clearDeleteError();
		const result = await deleteCombo(target.id);
		deleting = null;

		if (!result.ok) {
			// The code is what decides the dialog's lead sentence: only a CONFLICT means an alias still
			// references this combo. The API's message names that alias, and the panel does not paraphrase it.
			deleteConflict = result.error.code === 'CONFLICT';
			deleteError = result.error.message;
			return;
		}

		pendingDelete = null;
		if (editing?.id === target.id) closeEditor();
		void load();
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<p class="text-sm text-[var(--color-text-muted)]">
			A combo is a model string that resolves to several upstream models.
		</p>
		{#if !showEditor}
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)]"
				onclick={() => {
					editing = null;
					creating = true;
				}}>New combo</button
			>
		{/if}
	</div>

	<RefreshControl onrefresh={load} />

	{#if showEditor}
		<ComboEditor
			combo={editing}
			{suggestions}
			onsaved={() => {
				closeEditor();
				void load();
			}}
			oncancel={closeEditor}
		/>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading combos" />
	{:else if error}
		<StateMessage kind="error" title="Combos could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if combos.length === 0}
		<StateMessage
			kind="empty"
			title="No combos yet"
			description="A combo is a model string that resolves to several upstream models."
		>
			{#snippet action()}
				{#if !showEditor}
					<button
						type="button"
						class="underline"
						onclick={() => {
							editing = null;
							creating = true;
						}}>Create the first combo</button
					>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<ComboTable
			{combos}
			{deleting}
			onedit={(combo) => {
				creating = false;
				editing = combo;
			}}
			ondelete={(combo) => {
				clearDeleteError();
				pendingDelete = combo;
			}}
		/>

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={pageNumber <= 1}
				onclick={() => {
					pageNumber -= 1;
					void load();
				}}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">Page {pageNumber} of {lastPage}</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={pageNumber >= lastPage}
				onclick={() => {
					pageNumber += 1;
					void load();
				}}>Next</button
			>
		</div>
	{/if}
</div>

<ComboDeleteDialog
	combo={pendingDelete}
	error={deleteError}
	conflict={deleteConflict}
	deleting={deleting !== null}
	onconfirm={confirmDelete}
	oncancel={() => {
		pendingDelete = null;
		clearDeleteError();
	}}
/>
