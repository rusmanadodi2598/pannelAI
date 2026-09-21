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
	import AddEndpointKeyForm from '$lib/components/AddEndpointKeyForm.svelte';
	import EndpointFieldsForm from '$lib/components/EndpointFieldsForm.svelte';
	import EndpointKeysTable from '$lib/components/EndpointKeysTable.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
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
		onchanged
	}: { entry: Endpoint | null; onclose: () => void; onchanged: () => void } = $props();

	let detail = $state<Endpoint | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let notice = $state<string | null>(null);

	let lastTest = $state<EndpointTestStatus | null>(null);
	let testing = $state<string | null>(null);

	// The countdown's clock (§6.2 asks for a countdown on `rate_limited_until`). A second is the smallest
	// unit it prints, so a faster tick would redraw without changing what anyone reads. It runs only while
	// the drawer is open: a closed drawer has nothing to count down, and a timer left behind for a table
	// that is not on screen is a leak.
	const TICK_MS = 1000;
	let now = $state(Date.now());

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
			error = result.error.message;
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
			notice = result.error.message;
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
			notice = result.error.message;
			return;
		}

		await refresh();
	}

	async function setKeyStatus(key: EndpointKey, status: 'active' | 'disabled'): Promise<void> {
		if (!detail) return;

		const result = await updateEndpointKey(detail.id, key.id, { status });

		if (!result.ok) {
			notice = result.error.message;
			return;
		}

		await refresh();
	}
</script>

<Modal title={entry ? `Endpoint ${entry.label}` : 'Endpoint'} open={entry !== null} {onclose}>
	{#if loading}
		<StateMessage kind="loading" title="Loading the endpoint" />
	{:else if error}
		<StateMessage kind="error" title="The endpoint could not be loaded" description={error} />
	{:else if detail}
		<div class="flex flex-col gap-5 text-sm">
			{#if notice}
				<p class="text-[var(--color-text-muted)]">{notice}</p>
			{/if}

			<EndpointFieldsForm endpoint={detail} onsaved={saved} />

			<div class="flex flex-wrap items-center gap-3">
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 disabled:opacity-50"
					disabled={testing !== null}
					onclick={() => runTest()}>Test endpoint</button
				>
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
					<span
						>No key can be spent: every key is disabled, unhealthy, or absent. Requests to this
						endpoint will fail over or fail.</span
					>
				{/if}
			</div>

			{#if keys.length === 0}
				<StateMessage
					kind="empty"
					title="No keys on this endpoint"
					description="Add a credential so the router has something to spend."
				/>
			{:else}
				<EndpointKeysTable
					{keys}
					authType={detail.auth_type}
					{testing}
					{now}
					ontest={(key) => runTest(key.id)}
					onsettled={setKeyStatus}
					onremove={removeKey}
				/>
			{/if}

			<AddEndpointKeyForm endpointId={detail.id} onadded={refresh} />
		</div>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={onclose}>Close</button
		>
	{/snippet}
</Modal>
