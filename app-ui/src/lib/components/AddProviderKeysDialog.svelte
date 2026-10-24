<script lang="ts">
	// Add credentials to this provider without leaving its detail screen
	// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
	//
	// The concept is the reference's (`providers/[id]/AddApiKeyModal.js` at 9router origin/master): the
	// provider screen owns a list of connections, and a key added here becomes one of them. Single mode
	// asks for a name, the key, and a priority; bulk mode takes a paste, one key per line, and
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
	// The reference's Check button has no route here: app-serv probes a stored endpoint, and a credential
	// cannot be probed before the connection carrying it exists.
	import { untrack } from 'svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import ProviderKeyFields from '$lib/components/ProviderKeyFields.svelte';
	import { addConnection, addPastedConnections, readStoredLabels } from '$lib/api/provider-keys';
	import type { ApiError } from '$lib/api/errors';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';
	import { planConnectionLines } from '$lib/schemas/connection-plan';
	import { MAX_BULK_CONNECTIONS } from '$lib/schemas/endpoint-write';
	import { SERVICE_ACCOUNT_AUTH_TYPE } from '$lib/schemas/endpoint-credential';

	let {
		providerId,
		providerName,
		authType,
		authHint = null,
		credentialLabel = 'API Key',
		open,
		onadded,
		onclose
	}: {
		providerId: string;
		/** The registry's own name for the provider, which is what the title says the key is added to. */
		providerName: string;
		/** The auth type the connection is stored as: a key type, since a key is what this dialog takes. */
		authType: string;
		/**
		 * The registry's credential-format sentence (what a Personal Access Token looks like and where to
		 * mint one). Shown beside the field the operator pastes into, so the format is read where it is
		 * acted on, not only back on the section. Text, never a link.
		 */
		authHint?: string | null;
		/**
		 * What the credential field is called. 'API Key' for a key-only provider; 'Personal Access Token'
		 * for one that is OAuth with a key mode too (Qoder). The page derives it from the shared rule.
		 */
		credentialLabel?: string;
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
	let projectId = $state('');
	let location = $state('');
	let bulkText = $state('');
	let issues = $state<string[]>([]);
	let rowIssues = $state<Record<number, string>>({});
	let error = $state<string | ApiError | null>(null);
	let errorPrefix = $state('');
	let saving = $state(false);

	// A service-account credential is a JSON document, and the paste planner reads one credential per line,
	// so the batch tab is not offered for it. The mode is derived from the tabs actually shown, so a mode
	// left over from another provider's dialog cannot submit through a tab that is not on screen.
	const serviceAccount = $derived(authType === SERVICE_ACCOUNT_AUTH_TYPE);
	const tabs = $derived(serviceAccount ? [TABS[0]] : TABS);
	const activeMode = $derived(serviceAccount ? 'single' : mode);

	// The names already stored, so a generated name cannot collide with one of them: (provider_id, label)
	// is unique, and a collision refuses the whole paste. Read when the dialog opens rather than held by
	// the page, because a connection added from another screen since the page loaded is exactly the name a
	// stale list would miss.
	let existingLabels = $state<string[]>([]);

	const lines = $derived(planConnectionLines(bulkText, existingLabels));

	const empty = $derived(
		activeMode === 'single' ? name.trim() === '' || keyValue.trim() === '' : lines.length === 0
	);

	const submitLabel = $derived(
		activeMode === 'single' ? (saving ? 'Saving' : 'Save') : saving ? 'Adding' : 'Add All Keys'
	);

	$effect(() => {
		if (!open) return;
		untrack(() => void readLabels());
	});

	async function readLabels(): Promise<void> {
		existingLabels = await readStoredLabels(providerId);
	}

	// The dialog stays mounted between opens, so anything kept is the last connection's name sitting in the
	// next one's field. Priority stays: it is visible, and an operator adding several keys at one priority
	// means it. The service-account pair stays for the same reason: several JSON keys of one project is the
	// ordinary case.
	function clear(): void {
		name = '';
		keyValue = '';
		bulkText = '';
		rowIssues = {};
	}

	async function submit(): Promise<void> {
		issues = [];
		error = null;
		errorPrefix = '';

		saving = true;
		const outcome =
			activeMode === 'single'
				? await addConnection(
						providerId,
						authType,
						{ name, keyValue, priority },
						serviceAccount ? { sa_project_id: projectId, sa_location: location } : null
					)
				: await addPastedConnections(providerId, authType, bulkText, existingLabels);
		saving = false;

		if (outcome.kind === 'issues') {
			issues = outcome.issues;
			return;
		}
		if (outcome.kind === 'refused') {
			rowIssues = outcome.rowIssues;
			error = outcome.error;
			errorPrefix = mode === 'bulk' && outcome.error ? 'Nothing was added: ' : '';
			return;
		}

		clear();
		onadded({ count: outcome.count, label: outcome.label });
		onclose();
	}
</script>

<Modal title={`Add ${providerName} ${credentialLabel}`} {open} {onclose}>
	<div class="flex flex-col gap-3">
		{#if open && authHint}
			<p class="text-sm text-[var(--color-text-muted)]">{authHint}</p>
		{/if}
		<PanelTabs
			{tabs}
			label="How many keys"
			onchange={(id) => {
				mode = id;
				rowIssues = {};
			}}
		>
			{#snippet panel(active)}
				<ProviderKeyFields
					mode={active}
					{authType}
					{credentialLabel}
					bind:name
					bind:keyValue
					bind:priority
					bind:projectId
					bind:location
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
			<p class="text-sm text-[var(--color-danger)]" role="alert">
				<ApiErrorText {error} prefix={errorPrefix} />
			</p>
		{/if}
	</div>

	{#snippet footer()}
		<Button variant="ghost" class="underline" onclick={onclose}>
			<CancelIcon class="size-4" aria-hidden="true" />
			Cancel
		</Button>
		<Button
			variant="outline"
			disabled={saving || empty}
			aria-busy={saving}
			onclick={() => void submit()}
		>
			<SaveIcon class="size-4" aria-hidden="true" />
			{submitLabel}
		</Button>
	{/snippet}
</Modal>
