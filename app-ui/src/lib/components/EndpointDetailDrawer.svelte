<script lang="ts">
	// Endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2 detail).
	//
	// Three questions live here, in the order an operator asks them: what is this endpoint, which key would
	// route right now, and what are its keys doing. The routing answer comes from `$lib/utils/routing`,
	// derived from the fields the API returns rather than re-simulated, because a second router would
	// disagree with the real one the first time either changed.
	//
	// The editable fields, the credentials, and the routing answer are each their own component: the
	// credential half carries a selection, three bulk writes and a table of its own, which together are more
	// than one file may hold. This keeps the drawer's job: load the detail, test the endpoint, answer which
	// key would route, and coordinate the parts.
	import EndpointCredentials from '$lib/components/EndpointCredentials.svelte';
	import EndpointFieldsForm from '$lib/components/EndpointFieldsForm.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { getEndpoint, testEndpoint } from '$lib/api/endpoints';
	import { Button } from '$lib/primitives/button/index.js';
	import type { Endpoint, EndpointTestStatus } from '$lib/schemas/endpoint';
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
				<Button variant="outline" disabled={testing !== null} onclick={() => runTest()}>
					<TestIcon class="size-4" aria-hidden="true" />
					Test endpoint
				</Button>
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

			<EndpointCredentials
				endpoint={detail}
				{testing}
				{now}
				ontest={(key) => runTest(key.id)}
				onrefresh={refresh}
			/>
		</div>
	{/if}

	{#snippet footer()}
		{#if ondelete && detail}
			<Button variant="destructive" onclick={() => ondelete(detail!)}>
				<DeleteIcon class="size-4" aria-hidden="true" />
				Delete connection
			</Button>
		{/if}
		<Button variant="outline" onclick={onclose}>
			<CloseIcon class="size-4" aria-hidden="true" />
			Close
		</Button>
	{/snippet}
</Modal>
