<script lang="ts">
	// The Combos tab of /combos (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 1).
	//
	// The tab owns the page of combos, which one is being edited, and the delete confirmation. The table and
	// the editor are their own components, so this file holds the list's states and the two writes.
	//
	// The reference suggestions for the editor are the combos on this page and the refs of the providers
	// that are configured right now, which is the reference's own rule (`ModelSelectModal.js:216-219`). The
	// rule and the join live in `model-picker.ts`; the reads live in `model-picker-data.ts`, and the combos
	// half is derived here so a delete cannot leave a stale name in the picker.
	import { untrack } from 'svelte';
	import ComboDeleteDialog from '$lib/components/ComboDeleteDialog.svelte';
	import ComboEditor from '$lib/components/ComboEditor.svelte';
	import ComboTable from '$lib/components/ComboTable.svelte';
	import CombosToolbar from '$lib/components/CombosToolbar.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { CONTROL_ICONS } from '$lib/icons';
	import { deleteCombo, listCombos } from '$lib/api/combos';
	import { loadPickerSources } from '$lib/model-picker-data';
	import { pickerSections } from '$lib/schemas/model-picker';
	import type { Combo } from '$lib/schemas/combo';
	import type { CatalogModel } from '$lib/schemas/model';
	import type { Provider } from '$lib/schemas/provider';

	const AddIcon = CONTROL_ICONS.add.icon;
	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;
	const RetryIcon = CONTROL_ICONS.refresh.icon;

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

	let catalog = $state<CatalogModel[]>([]);
	let providers = $state<Provider[]>([]);
	let pickerFailed = $state(false);
	// The picker's sources load beside the combos list, so the editor can be opened before they arrive.
	// The dialog is told, rather than rendering its empty sentence over a read still in flight (R-27).
	let pickerLoading = $state(true);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const showEditor = $derived(creating || editing !== null);

	// The editor's picker sections: the combos on this page, then the refs of the providers that carry an
	// endpoint right now. Derived rather than stored, so a delete or a page change cannot leave a name the
	// picker still offers behind.
	const sections = $derived(
		pickerSections({ catalog, providers, combos: combos.map((combo) => combo.name) })
	);

	$effect(() => {
		untrack(() => {
			void load();
			void loadPicker();
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

	// The picker's sources are a convenience, so a failure here does not block the screen: the editor still
	// works with a typed reference, and the picker says its own read failed rather than showing an empty
	// catalog as the truth.
	async function loadPicker(): Promise<void> {
		const sources = await loadPickerSources({});
		catalog = sources.catalog;
		providers = sources.providers;
		pickerFailed = sources.failed;
		pickerLoading = false;
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
	<CombosToolbar
		hidden={showEditor}
		onrefresh={load}
		oncreate={() => {
			editing = null;
			creating = true;
		}}
	/>

	{#if showEditor}
		<ComboEditor
			combo={editing}
			{sections}
			{pickerLoading}
			{pickerFailed}
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
				<button type="button" class="inline-flex items-center gap-2 underline" onclick={load}>
					<RetryIcon class="size-4" aria-hidden="true" />
					Try again
				</button>
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
						class="inline-flex items-center gap-2 underline"
						onclick={() => {
							editing = null;
							creating = true;
						}}
					>
						<AddIcon class="size-4" aria-hidden="true" />
						Create the first combo
					</button>
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
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={pageNumber <= 1}
				onclick={() => {
					pageNumber -= 1;
					void load();
				}}
			>
				<PreviousIcon class="size-4" aria-hidden="true" />
				Previous
			</button>
			<span class="text-[var(--color-text-muted)]">Page {pageNumber} of {lastPage}</span>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={pageNumber >= lastPage}
				onclick={() => {
					pageNumber += 1;
					void load();
				}}
			>
				Next
				<NextIcon class="size-4" aria-hidden="true" />
			</button>
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
