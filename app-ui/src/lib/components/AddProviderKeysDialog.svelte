<script lang="ts">
	// Add credentials to this provider without leaving its detail screen
	// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
	//
	// The concept is the reference's (`providers/[id]/AddApiKeyModal.js` at 9router origin/master): the
	// provider screen owns a list of connections, and a key added here becomes one of them. Single mode
	// asks for a name, the key, and a priority (`:229-403`); bulk mode takes a paste, one key per line, and
	// plans a name for each line (`shared/utils/bulkAdd.js`). A connection is what this panel's API calls
	// an endpoint of the provider, so the single case is one `POST /endpoints` and the paste is one
	// `POST /endpoints/bulk`: three pasted keys become three connections of this provider, never one
	// connection holding three keys.
	//
	// The two write flows live in `$lib/api/provider-keys`, so this component owns the dialog's own state
	// (which tab is showing, what was typed, what the last attempt answered) and nothing about the wire.
	// A paste is all-or-nothing (SPEC-API §8.1), and the flow puts the server's refusal back on the line
	// the operator pasted.
	//
	// The reference's Check button (`:263-266`) has no route here: app-serv probes a stored endpoint, and a
	// credential cannot be probed before the connection carrying it exists. The gap is filed as
	// `docs/DRAFT/017-PROVIDER-PARITY-READINESS.md §4.6` (F6).
	import { untrack } from 'svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import ProviderKeyFields from '$lib/components/ProviderKeyFields.svelte';
	import { addConnection, addPastedConnections } from '$lib/api/provider-keys';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { listEndpointLabels } from '$lib/api/endpoints';
	import { planConnectionLines } from '$lib/schemas/connection-plan';
	import { MAX_BULK_CONNECTIONS } from '$lib/schemas/endpoint-write';

	let {
		providerId,
		providerName,
		authType,
		open,
		onadded,
		onclose
	}: {
		providerId: string;
		/** The registry's own name for the provider, which is what the title says the key is added to. */
		providerName: string;
		/** The provider's own auth type, which is the connection's: a key is only asked for where it applies. */
		authType: string;
		open: boolean;
		/** Hands back what was stored, so the section can say it (§8.6.3). */
		onadded: (added: { count: number; label: string | null }) => void;
		onclose: () => void;
	} = $props();

	// The footer's two controls: Cancel backs out, the other commits one key or the whole paste. Both
	// keep their label and gain the glyph beside it (PORT 002 D4).
	const SaveIcon = ROW_ACTION_ICONS.save.icon;
	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;

	const TABS = [
		{ id: 'single', label: 'Single' },
		{ id: 'bulk', label: 'Bulk Add' }
	];

	// Which tab is showing, mirrored out of `PanelTabs` through its `onchange`. It has to be state here
	// rather than read off the rendered markup: the submit branches on it, and a selection the parent
	// cannot see is a mode whose fields are on screen but never submitted.
	let mode = $state('single');
	let name = $state('');
	let keyValue = $state('');
	let priority = $state('1');
	let bulkText = $state('');
	let issues = $state<string[]>([]);
	let rowIssues = $state<Record<number, string>>({});
	let error = $state<string | null>(null);
	let saving = $state(false);

	// The names already stored, so a generated name cannot collide with one of them: (provider_id, label)
	// is unique, and a collision refuses the whole paste. Read when the dialog opens rather than held by
	// the page, because a connection added from another screen since the page loaded is exactly the name a
	// stale list would miss.
	let existingLabels = $state<string[]>([]);

	const lines = $derived(planConnectionLines(bulkText, existingLabels));

	const empty = $derived(
		mode === 'single' ? name.trim() === '' || keyValue.trim() === '' : lines.length === 0
	);

	const submitLabel = $derived(
		mode === 'single' ? (saving ? 'Saving' : 'Save') : saving ? 'Adding' : 'Add All Keys'
	);

	$effect(() => {
		if (!open) return;
		untrack(() => void readLabels());
	});

	async function readLabels(): Promise<void> {
		const result = await listEndpointLabels({ provider_id: providerId, page: 1, per_page: 100 });
		// A failed read plans against no names. The planner then cannot see a collision, and the batch's own
		// refusal is what catches it, on the line it belongs to.
		existingLabels = result.ok ? result.data.data.map((row) => row.label) : [];
	}

	// The dialog stays mounted between opens, so anything kept is the last connection's name sitting in the
	// next one's field. Priority stays: it is visible, and an operator adding several keys at one priority
	// means it.
	function clear(): void {
		name = '';
		keyValue = '';
		bulkText = '';
		rowIssues = {};
	}

	async function submit(): Promise<void> {
		issues = [];
		error = null;

		saving = true;
		const outcome =
			mode === 'single'
				? await addConnection(providerId, authType, { name, keyValue, priority })
				: await addPastedConnections(providerId, authType, bulkText, existingLabels);
		saving = false;

		if (outcome.kind === 'issues') {
			issues = outcome.issues;
			return;
		}
		if (outcome.kind === 'refused') {
			rowIssues = outcome.rowIssues;
			error = outcome.error;
			return;
		}

		clear();
		onadded({ count: outcome.count, label: outcome.label });
		onclose();
	}
</script>

<Modal title={`Add ${providerName} API Key`} {open} {onclose}>
	<div class="flex flex-col gap-3">
		<PanelTabs
			tabs={TABS}
			label="How many keys"
			onchange={(id) => {
				mode = id;
				rowIssues = {};
			}}
		>
			{#snippet panel(active)}
				<ProviderKeyFields
					mode={active}
					bind:name
					bind:keyValue
					bind:priority
					bind:pasteText={bulkText}
					{rowIssues}
					maxConnections={MAX_BULK_CONNECTIONS}
					onedit={() => {
						rowIssues = {};
					}}
				/>
			{/snippet}
		</PanelTabs>

		<FormIssues {issues} />

		{#if error}
			<p class="text-sm text-[var(--color-danger)]" role="alert">{error}</p>
		{/if}
	</div>

	{#snippet footer()}
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 underline"
			onclick={onclose}
		>
			<CancelIcon class="size-4" aria-hidden="true" />
			Cancel
		</button>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
			disabled={saving || empty}
			onclick={() => void submit()}
		>
			<SaveIcon class="size-4" aria-hidden="true" />
			{submitLabel}
		</button>
	{/snippet}
</Modal>
