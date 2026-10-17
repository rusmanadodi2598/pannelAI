<script lang="ts">
	// Endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2 detail).
	//
	// Three questions live here, in the order an operator asks them: what is this endpoint, which key would
	// route right now, and what are its keys doing. The routing answer comes from `$lib/utils/routing`,
	// derived from the fields the API returns rather than re-simulated, because a second router would
	// disagree with the real one the first time either changed.
	//
	// The editable fields, the keys table, and the add-key form are each their own component. Together they
	// carry more state and markup than one file is allowed, so this keeps the drawer's own job: load the
	// detail, answer the routing question, and coordinate the parts.
	import AddEndpointKeysPanel from '$lib/components/AddEndpointKeysPanel.svelte';
	import EndpointFieldsForm from '$lib/components/EndpointFieldsForm.svelte';
	import BulkActionBar from '$lib/components/BulkActionBar.svelte';
	import EndpointKeysTable from '$lib/components/EndpointKeysTable.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { emptySelection, toggleAll, toggleRow, type Selection } from '$lib/utils/selection';
	import { bulkDelete, bulkStatus, bulkTags } from '$lib/api/bulk-mutation';
	import {
		deleteEndpointKey,
		getEndpoint,
		testEndpoint,
		updateEndpointKey
	} from '$lib/api/endpoints';
	import type { Endpoint, EndpointKey, EndpointTestStatus } from '$lib/schemas/endpoint';
	import { routingKey } from '$lib/utils/routing';

	let {
		entry,
		onclose,
		onchanged,
		ondelete
	}: {
		entry: Endpoint | null;
		onclose: () => void;
		onchanged: () => void;
		/**
		 * Present when the list this drawer sits in can remove the endpoint it opened. The confirmation is the
		 * caller's (one dialog for row and drawer alike), so this only hands the endpoint up and lets the
		 * caller close the drawer with it.
		 */
		ondelete?: (endpoint: Endpoint) => void;
	} = $props();

	let detail = $state<Endpoint | null>(null);
	let loading = $state(false);
	let error = $state<ApiError | null>(null);
	let notice = $state<ApiError | null>(null);

	let lastTest = $state<EndpointTestStatus | null>(null);
	let testing = $state<string | null>(null);

	// The countdown's clock (§6.2 asks for a countdown on `rate_limited_until`). A second is the smallest
	// unit it prints, so a faster tick would redraw without changing what anyone reads. It runs only while
	// the drawer is open: a closed drawer has nothing to count down, and a timer left behind for a table
	// that is not on screen is a leak.
	const TICK_MS = 1000;
	let now = $state(Date.now());

	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const CloseIcon = CONTROL_ICONS.clear.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	const keys = $derived(detail?.keys ?? []);
	const chosen = $derived(routingKey(keys));

	$effect(() => {
		if (!entry) return;

		const timer = setInterval(() => (now = Date.now()), TICK_MS);
		return () => clearInterval(timer);
	});

	// Reload whenever the drawer is handed a different endpoint, so the keys and the routing answer always
	// belong to the row that was clicked.
	$effect(() => {
		const id = entry?.id;
		if (!id) {
			detail = null;
			return;
		}
		void load(id);
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const result = await getEndpoint(id);
		loading = false;

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		detail = result.data;
	}

	async function refresh(): Promise<void> {
		if (!detail) return;
		await load(detail.id);
		onchanged();
	}

	function saved(updated: Endpoint): void {
		detail = updated;
		onchanged();
	}

	async function runTest(keyId?: string): Promise<void> {
		if (!detail) return;

		testing = keyId ?? detail.id;
		const result = await testEndpoint(detail.id, keyId);
		testing = null;

		if (!result.ok) {
			notice = result.error;
			return;
		}

		lastTest = result.data;
		await refresh();
	}

	async function removeKey(key: EndpointKey): Promise<void> {
		if (!detail) return;

		const result = await deleteEndpointKey(detail.id, key.id);

		if (!result.ok) {
			// The API answers CONFLICT for the last active api_key key. The control is disabled for that
			// case, but the state can change under the panel, so the response is handled as well.
			notice = result.error;
			return;
		}

		await refresh();
	}

	async function setKeyStatus(key: EndpointKey, status: 'active' | 'disabled'): Promise<void> {
		if (!detail) return;

		const result = await updateEndpointKey(detail.id, key.id, { status });

		if (!result.ok) {
			notice = result.error;
			return;
		}

		await refresh();
	}

	// Selection here addresses the credentials of this one connection. Select-All therefore means
	// "every key this endpoint holds" and never a cross-provider set: the drawer has no server scope
	// for that, and a control promising one would be counting rows it cannot see.
	let picked = $state<Selection>(emptySelection);
	let bulkBusy = $state(false);

	const keyIds = $derived(keys.map((key) => key.id));
	const bulkCount = $derived(picked.all ? keyIds.length : picked.ids.length);
	const bulkScope = $derived(
		picked.all
			? `all ${keyIds.length} keys on this connection`
			: `${picked.ids.length} named by you`
	);

	function toggleChosen(id: string, checked: boolean): void {
		picked = toggleRow(picked, id, checked, keyIds);
	}

	function toggleChooseAll(checked: boolean): void {
		picked = toggleAll(checked);
	}

	function bulkSelector(): { ids?: string[]; all?: true } {
		return picked.all ? { all: true } : { ids: picked.ids };
	}

	async function runBulkStatus(status: 'active' | 'disabled'): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkStatus('endpoint-keys', bulkSelector(), status, bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			notice = result.error;
			return;
		}
		notice = null;
		await refresh();
	}

	async function runBulkTag(tag: string): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkTags('endpoint-keys', bulkSelector(), [tag], [], false, bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			notice = result.error;
			return;
		}
		notice = null;
		await refresh();
	}

	async function runBulkDelete(): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkDelete('endpoint-keys', bulkSelector(), bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			// A refused row is the account's floor: an endpoint keeps at least one usable
			// credential, and the refusal names which key would have emptied it.
			notice = result.error;
			return;
		}
		notice = null;
		picked = emptySelection;
		await refresh();
	}
</script>

<Modal title={entry ? `Endpoint ${entry.label}` : 'Endpoint'} open={entry !== null} {onclose}>
	{#if loading}
		<StateMessage kind="loading" title="Loading the endpoint" />
	{:else if error}
		<StateMessage kind="error" title="The endpoint could not be loaded" apiError={error} />
	{:else if detail}
		<div class="flex flex-col gap-5 text-sm">
			{#if notice}
				<p class="text-[var(--color-text-muted)]"><ApiErrorText error={notice} /></p>
			{/if}

			<EndpointFieldsForm endpoint={detail} onsaved={saved} />

			<div class="flex flex-wrap items-center gap-3">
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 disabled:opacity-50"
					disabled={testing !== null}
					onclick={() => runTest()}
				>
					<TestIcon class="size-4" aria-hidden="true" />
					Test endpoint
				</button>
				{#if lastTest}
					<span>
						Test result: <span class="font-medium">{lastTest.state}</span>
						{lastTest.latency_ms}ms{lastTest.message ? `, ${lastTest.message}` : ''}
					</span>
				{/if}
			</div>

			<div class="flex flex-col gap-1 rounded-[var(--radius-md)] bg-[var(--color-surface-2)] p-3">
				<span class="font-medium">Which key would route now</span>
				{#if chosen}
					<span
						>{chosen.label || 'Unlabelled key'} ({chosen.key_hint}), priority {chosen.priority}</span
					>
				{:else}
					<span>Every key is disabled, unhealthy, or absent. Fix one to route.</span>
				{/if}
			</div>

			{#if keys.length === 0}
				<StateMessage
					kind="empty"
					title="No keys on this endpoint"
					description="Add a credential so requests can route."
				/>
			{:else}
				<EndpointKeysTable
					{keys}
					authType={detail.auth_type}
					{testing}
					{now}
					selected={picked.ids}
					allSelected={picked.all}
					ontest={(key) => runTest(key.id)}
					onsettled={setKeyStatus}
					onremove={removeKey}
					onselect={toggleChosen}
				/>

				<BulkActionBar
					kind="keys"
					selected={bulkCount}
					total={keyIds.length}
					scope={bulkScope}
					allSelected={picked.all}
					ontag={(tag: string) => void runBulkTag(tag)}
					ondisable={() => void runBulkStatus('disabled')}
					onenable={() => void runBulkStatus('active')}
					ondelete={() => void runBulkDelete()}
					onclear={() => (picked = emptySelection)}
					ontoggleall={toggleChooseAll}
				/>
				{#if bulkBusy}
					<p role="status" class="text-sm text-[var(--color-text-muted)]">
						Applying to the selected keys...
					</p>
				{/if}
			{/if}

			<AddEndpointKeysPanel endpointId={detail.id} onadded={refresh} />
		</div>
	{/if}

	{#snippet footer()}
		{#if ondelete && detail}
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-sm text-[var(--color-accent-text)]"
				onclick={() => ondelete(detail!)}
			>
				<DeleteIcon class="size-4" aria-hidden="true" />
				Delete connection
			</button>
		{/if}
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={onclose}
		>
			<CloseIcon class="size-4" aria-hidden="true" />
			Close
		</button>
	{/snippet}
</Modal>
