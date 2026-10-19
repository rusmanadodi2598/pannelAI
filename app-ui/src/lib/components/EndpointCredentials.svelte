<script lang="ts">
	// The credential half of the endpoint drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2): the keys one connection
	// holds, what can be done to one of them, and what can be done to the set the operator selected.
	//
	// Selection here addresses this connection's keys and never a cross-provider set. `all` therefore means
	// every key the endpoint holds: the drawer has no server scope for more, and a control promising more
	// would be counting rows it cannot see.
	import AddEndpointKeysPanel from '$lib/components/AddEndpointKeysPanel.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import BulkActionBar from '$lib/components/BulkActionBar.svelte';
	import EndpointKeysTable from '$lib/components/EndpointKeysTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { deleteEndpointKey, updateEndpointKey } from '$lib/api/endpoints';
	import type { ApiError } from '$lib/api/errors';
	import { createBulkActions } from '$lib/stores/bulk-actions.svelte';
	import type { Endpoint, EndpointKey } from '$lib/schemas/endpoint';

	let {
		endpoint,
		testing,
		now,
		ontest,
		onrefresh
	}: {
		endpoint: Endpoint;
		/** The id of the key or endpoint under a test, which is what disables every row test meanwhile. */
		testing: string | null;
		/** The drawer's clock, so a countdown and the elapsed columns run against one instant. */
		now: number;
		ontest: (key: EndpointKey) => void;
		onrefresh: () => Promise<void> | void;
	} = $props();

	const keys = $derived(endpoint.keys ?? []);
	let rowFailure = $state<ApiError | null>(null);

	const bulk = createBulkActions({
		kind: 'endpoint-keys',
		ids: () => keys.map((key) => key.id),
		total: () => keys.length,
		onapplied: () => onrefresh()
	});

	const scope = $derived(
		bulk.selected.all
			? `all ${keys.length} keys on this connection`
			: `${bulk.selected.ids.length} named by you`
	);

	// A refusal on one credential reads under the table rather than at the top of the drawer: the operator
	// is looking at the row they clicked, and the drawer's own notice carries the endpoint test beside it.
	const failure = $derived(bulk.error ?? rowFailure);

	async function removeKey(key: EndpointKey): Promise<void> {
		// The API answers CONFLICT for the last active api_key key. The control is disabled for that case,
		// but the state can change under the panel, so the response is handled as well.
		const result = await deleteEndpointKey(endpoint.id, key.id);
		rowFailure = result.ok ? null : result.error;
		if (result.ok) await onrefresh();
	}

	async function setKeyStatus(key: EndpointKey, status: 'active' | 'disabled'): Promise<void> {
		const result = await updateEndpointKey(endpoint.id, key.id, { status });
		rowFailure = result.ok ? null : result.error;
		if (result.ok) await onrefresh();
	}
</script>

{#if keys.length === 0}
	<StateMessage
		kind="empty"
		title="No keys on this endpoint"
		description="Add a credential so requests can route."
	/>
{:else}
	<EndpointKeysTable
		{keys}
		authType={endpoint.auth_type}
		{testing}
		{now}
		{ontest}
		selected={bulk.selected.ids}
		allSelected={bulk.selected.all}
		onsettled={(key, status) => void setKeyStatus(key, status)}
		onremove={(key) => void removeKey(key)}
		onselect={(id, checked) => bulk.toggle(id, checked)}
	/>

	<BulkActionBar
		kind="keys"
		selected={bulk.count}
		total={keys.length}
		{scope}
		allSelected={bulk.selected.all}
		ontag={(tag: string) => void bulk.tag(tag)}
		ondisable={() => void bulk.setStatus('disabled')}
		onenable={() => void bulk.setStatus('active')}
		ondelete={() => void bulk.remove()}
		onclear={() => bulk.clear()}
		ontoggleall={(checked: boolean) => bulk.toggleEvery(checked)}
	/>

	{#if bulk.busy}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">
			Applying to the selected keys...
		</p>
	{/if}
	{#if failure}
		<!-- A refused row is the account's floor: an endpoint keeps at least one usable credential, and the
			 refusal names which key would have emptied it. -->
		<p role="alert" class="text-sm text-[var(--color-danger)]"><ApiErrorText error={failure} /></p>
	{/if}
{/if}

<AddEndpointKeysPanel endpointId={endpoint.id} onadded={() => void onrefresh()} />
