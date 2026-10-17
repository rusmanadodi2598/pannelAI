<script lang="ts">
	// Proxy pools (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// The screen owns the pool and re-reads it after every write, including a connectivity test,
	// because a test stores its result: the row's "last test" column is what the API recorded, not
	// what this page saw. §8.6.3 is why the write path ends in `load()` rather than in a local patch.
	//
	// The empty state's sentence is the one SPEC-API §7.11 supports: the pool rows route upstream calls, not
	// only the global `network.outbound_proxy_url` (docs/PORT/008-PORT-PROXY-ENGINE.md D1), so "Add one to
	// route upstream calls through it" is true and SPEC-UI §14 Q15 records the resolution.
	//
	// The outbound settings are their own card with their own load, so a settings failure cannot stop
	// the pool from rendering. §6.13 links here from Settings instead of duplicating that form.
	import { onMount } from 'svelte';
	import ProxyBatchAdd from '$lib/components/ProxyBatchAdd.svelte';
	import ProxyDeleteDialog from '$lib/components/ProxyDeleteDialog.svelte';
	import ProxyFormModal from '$lib/components/ProxyFormModal.svelte';
	import ProxyOutboundSettings from '$lib/components/ProxyOutboundSettings.svelte';
	import ProxyTable from '$lib/components/ProxyTable.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { emptySelection, toggleAll, toggleRow, type Selection } from '$lib/utils/selection';
	import { CONTROL_ICONS } from '$lib/icons';
	import { deleteProxy, listProxies, testProxy, updateProxy } from '$lib/api/proxies';
	import { bulkProxyDelete, bulkProxyPreview, bulkProxyStatus } from '$lib/api/bulk-mutation';
	import BulkActionBar from '$lib/components/BulkActionBar.svelte';
	import { proxyTestStateLabel, type Proxy, type ProxyForm } from '$lib/schemas/proxy';

	const AddIcon = CONTROL_ICONS.add.icon;
	const AddManyIcon = CONTROL_ICONS.addMany.icon;

	let proxies = $state<Proxy[] | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	/** The row the form is open on, `'new'` for an add, or null when it is shut. */
	let formTarget = $state<Proxy | 'new' | null>(null);
	let deleting = $state<Proxy | null>(null);
	let deleteError = $state<ApiError | null>(null);
	let deletingBusy = $state(false);

	let testingId = $state<string | null>(null);
	let notice = $state<string | ApiError | null>(null);
	let noticePrefix = $state('');
	let showBatch = $state(false);

	// Selection is a set of ids and a claim about the server's count. `allSelected` means the
	// operator asked for every candidate the list reports, so the request carries `all` and the
	// server resolves it - never the twelve rows this page happens to hold.
	let selected = $state<Selection>(emptySelection);
	let bulkError = $state<ApiError | null>(null);
	let bulkBusy = $state(false);

	const total = $derived(proxies?.length ?? 0);

	function toggleSelected(id: string, checked: boolean): void {
		selected = toggleRow(
			selected,
			id,
			checked,
			(proxies ?? []).map((proxy) => proxy.id)
		);
	}

	function toggleEvery(checked: boolean): void {
		selected = toggleAll(checked);
	}

	/** The selector the bulk routes need: the whole set when Select-All is on, the named ids otherwise. */
	function bulkSelector(): { ids?: string[]; all?: true } {
		return selected.all ? { all: true } : { ids: selected.ids };
	}

	/** How many rows the action covers, which every bulk body has to confirm. */
	const bulkCount = $derived(selected.all ? total : selected.ids.length);

	const bulkScope = $derived(
		selected.all ? `all ${total} candidates on the server` : `${selected.ids.length} named by you`
	);

	function formFor(proxy: Proxy, enabled: boolean): ProxyForm {
		return {
			label: proxy.label,
			protocol: proxy.protocol,
			host: proxy.host,
			port: proxy.port,
			username: proxy.username,
			// An empty password keeps the stored secret: the wire carries `has_password` and never
			// the value, so a switch cannot restate a credential it was not given.
			password: '',
			enabled
		};
	}

	async function toggleEnabled(proxy: Proxy, enabled: boolean): Promise<void> {
		const result = await updateProxy(proxy.id, formFor(proxy, enabled));
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		await load();
	}

	async function runBulk(enabled: boolean): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const result = await bulkProxyStatus(bulkSelector(), enabled, bulkCount);
		bulkBusy = false;
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		await load();
	}

	// The erase is confirmed against the number storage reports, not a number this page guesses:
	// a candidate is pinned by an id string, so only the server knows which connections lose their
	// egress path. Ask, show the answer, then erase with that figure echoed back.
	async function runBulkDelete(): Promise<void> {
		if (bulkCount === 0) return;
		bulkBusy = true;
		const preview = await bulkProxyPreview(bulkSelector(), bulkCount);
		if (!preview.ok) {
			bulkBusy = false;
			bulkError = preview.error;
			return;
		}
		const result = await bulkProxyDelete(bulkSelector(), bulkCount, preview.data.pins.endpoints);
		bulkBusy = false;
		if (!result.ok) {
			bulkError = result.error;
			return;
		}
		bulkError = null;
		noticePrefix = 'Pool';
		notice = `Removed ${result.data.changed} candidates. ${result.data.pins.endpoints} endpoint pin(s) across ${result.data.pins.providers.length} provider(s) went with them.`;
		toggleAll(false);
		await load();
	}

	onMount(() => void load());

	async function load(): Promise<void> {
		loading = true;
		const result = await listProxies();
		loading = false;

		if (!result.ok) {
			error = result.error;
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
			notice = answer.error;
			noticePrefix = `${proxy.label} could not be tested: `;
			return;
		}

		// A fail state is an answer, so it is announced as one with the reason the gateway reported.
		const state = proxyTestStateLabel(answer.data.state);
		const reason = answer.data.message ? ` ${answer.data.message}` : '';
		notice = `${proxy.label}: ${state} in ${answer.data.latency_ms}ms.${reason}`;
		noticePrefix = '';

		await load();
	}

	async function confirmDelete(): Promise<void> {
		if (deleting === null) return;

		deleteError = null;
		deletingBusy = true;
		const answer = await deleteProxy(deleting.id);
		deletingBusy = false;

		if (!answer.ok) {
			deleteError = answer.error;
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
			Addresses you can test before routing through them.
		</p>
	</div>

	<RefreshControl onrefresh={load} />

	{#if loading && proxies === null}
		<StateMessage kind="loading" title="Loading the proxy pool" />
	{:else if error && proxies === null}
		<StateMessage kind="error" title="Proxy pool not loaded" apiError={error}>
			{#snippet action()}
				<button
					type="button"
					class="inline-flex min-h-11 items-center px-2 underline"
					onclick={() => void load()}>Try again</button
				>
			{/snippet}
		</StateMessage>
	{:else if proxies}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				<ApiErrorText {error} suffix=" The table still shows the last read." />
			</p>
		{/if}

		{#if notice}
			<p role="status" class="text-sm text-[var(--color-text-muted)]">
				<ApiErrorText error={notice} prefix={noticePrefix} />
			</p>
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
			<!-- Refresh lives only in the shared control above: a second
			     plain "Refresh" here would be the same action written twice, and the shared control renders in
			     every state this toolbar can appear in. -->
		</div>

		{#if showBatch}
			<ProxyBatchAdd onsubmitted={load} />
		{/if}

		{#if proxies.length === 0}
			<StateMessage
				kind="empty"
				title="No proxies yet"
				description="Add one to route upstream calls."
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
				selected={selected.ids}
				allSelected={selected.all}
				{total}
				onedit={(proxy) => (formTarget = proxy)}
				ontest={(proxy) => void runTest(proxy)}
				ontoggle={(proxy, enabled) => void toggleEnabled(proxy, enabled)}
				onselect={toggleSelected}
				ondelete={(proxy) => {
					deleteError = null;
					deleting = proxy;
				}}
			/>

			<BulkActionBar
				kind="candidates"
				selected={bulkCount}
				{total}
				scope={bulkScope}
				allSelected={selected.all}
				ondisable={() => void runBulk(false)}
				onenable={() => void runBulk(true)}
				ondelete={() => void runBulkDelete()}
				onclear={() => (selected = emptySelection)}
				ontoggleall={toggleEvery}
			/>

			{#if bulkError}
				<p role="alert" class="text-sm text-[var(--color-danger)]">
					<ApiErrorText error={bulkError} />
				</p>
			{/if}
			{#if bulkBusy}
				<p role="status" class="text-sm text-[var(--color-text-muted)]">
					Applying the bulk action...
				</p>
			{/if}
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
