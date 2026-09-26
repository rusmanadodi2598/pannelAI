<script lang="ts">
	// Proxy pools (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// The screen owns the pool and re-reads it after every write, including a connectivity test,
	// because a test stores its result: the row's "last test" column is what the API recorded, not
	// what this page saw. §8.6.3 is why the write path ends in `load()` rather than in a local patch.
	//
	// The empty state is not §6.9's sentence as written. The spec gives "Add one to route upstream
	// calls through it", and SPEC-API §7.11 says the opposite about what routes: the global
	// `network.outbound_proxy_url` carries traffic and a pool row is a stored candidate, which the
	// egress path confirms. The copy below says what is true, the outbound card says the same thing,
	// and the divergence is recorded as SPEC-UI §14 Q15 rather than quietly followed.
	//
	// The outbound settings are their own card with their own load, so a settings failure cannot stop
	// the pool from rendering. §6.13 links here from Settings instead of duplicating that form.
	import { onMount } from 'svelte';
	import ProxyBatchAdd from '$lib/components/ProxyBatchAdd.svelte';
	import ProxyDeleteDialog from '$lib/components/ProxyDeleteDialog.svelte';
	import ProxyFormModal from '$lib/components/ProxyFormModal.svelte';
	import ProxyOutboundSettings from '$lib/components/ProxyOutboundSettings.svelte';
	import ProxyTable from '$lib/components/ProxyTable.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { CONTROL_ICONS } from '$lib/icons';
	import { deleteProxy, listProxies, testProxy } from '$lib/api/proxies';
	import { proxyTestStateLabel, type Proxy } from '$lib/schemas/proxy';

	const AddIcon = CONTROL_ICONS.add.icon;
	const AddManyIcon = CONTROL_ICONS.addMany.icon;

	let proxies = $state<Proxy[] | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	/** The row the form is open on, `'new'` for an add, or null when it is shut. */
	let formTarget = $state<Proxy | 'new' | null>(null);
	let deleting = $state<Proxy | null>(null);
	let deleteError = $state<string | null>(null);
	let deletingBusy = $state(false);

	let testingId = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let showBatch = $state(false);

	onMount(() => void load());

	async function load(): Promise<void> {
		loading = true;
		const result = await listProxies();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		proxies = result.data.data;
	}

	async function runTest(proxy: Proxy): Promise<void> {
		notice = null;
		testingId = proxy.id;
		const answer = await testProxy(proxy.id);
		testingId = null;

		if (!answer.ok) {
			notice = `${proxy.label} could not be tested: ${answer.error.message}`;
			return;
		}

		// A fail state is an answer, so it is announced as one with the reason the gateway reported.
		const state = proxyTestStateLabel(answer.data.state);
		const reason = answer.data.message ? ` ${answer.data.message}` : '';
		notice = `${proxy.label}: ${state} in ${answer.data.latency_ms}ms.${reason}`;

		await load();
	}

	async function confirmDelete(): Promise<void> {
		if (deleting === null) return;

		deleteError = null;
		deletingBusy = true;
		const answer = await deleteProxy(deleting.id);
		deletingBusy = false;

		if (!answer.ok) {
			deleteError = answer.error.message;
			return;
		}

		deleting = null;
		await load();
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Proxy Pools</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			Addresses kept here so you can test one before putting it on the outbound path. The pool
			itself routes nothing.
		</p>
	</div>

	<RefreshControl onrefresh={load} />

	{#if loading && proxies === null}
		<StateMessage kind="loading" title="Loading the proxy pool" />
	{:else if error && proxies === null}
		<StateMessage kind="error" title="The proxy pool could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if proxies}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				{error} The table below still shows the last pool that was read.
			</p>
		{/if}

		{#if notice}
			<p role="status" class="text-sm text-[var(--color-text-muted)]">{notice}</p>
		{/if}

		<div class="flex flex-wrap items-center gap-3">
			{#if proxies.length > 0}
				<!-- The empty state owns the primary action while the pool is empty, so the toolbar does not
				     repeat it. Two buttons with one label is a redundant control, and an ambiguous one for
				     anyone reading the screen rather than looking at it. -->
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)]"
					onclick={() => (formTarget = 'new')}
				>
					<AddIcon class="size-4" aria-hidden="true" />
					Add a proxy
				</button>
			{/if}
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 underline"
				onclick={() => (showBatch = !showBatch)}
			>
				<AddManyIcon class="size-4" aria-hidden="true" />
				{showBatch ? 'Hide the paste box' : 'Add several at once'}
			</button>
			<!-- Refresh lives only in the shared control above (owner directive, 2026-09-26): a second
			     plain "Refresh" here was the same action written twice, and the shared control renders in
			     every state this toolbar can appear in. -->
		</div>

		{#if showBatch}
			<ProxyBatchAdd onsubmitted={load} />
		{/if}

		{#if proxies.length === 0}
			<StateMessage
				kind="empty"
				title="No proxies yet"
				description="Add one to keep and test an address before you point the outbound setting at it."
			>
				{#snippet action()}
					<button
						type="button"
						class="inline-flex min-h-11 items-center gap-2 underline"
						onclick={() => (formTarget = 'new')}
					>
						<AddIcon class="size-4" aria-hidden="true" />
						Add a proxy
					</button>
				{/snippet}
			</StateMessage>
		{:else}
			<ProxyTable
				{proxies}
				{testingId}
				onedit={(proxy) => (formTarget = proxy)}
				ontest={(proxy) => void runTest(proxy)}
				ondelete={(proxy) => {
					deleteError = null;
					deleting = proxy;
				}}
			/>
		{/if}

		<ProxyOutboundSettings />
	{/if}

	<ProxyFormModal target={formTarget} onsaved={load} onclose={() => (formTarget = null)} />
	<ProxyDeleteDialog
		proxy={deleting}
		error={deleteError}
		deleting={deletingBusy}
		onconfirm={() => void confirmDelete()}
		oncancel={() => {
			deleting = null;
			deleteError = null;
		}}
	/>
</section>
