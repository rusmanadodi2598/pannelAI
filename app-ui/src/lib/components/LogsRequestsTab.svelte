<script lang="ts">
	// The Requests tab of Logs (docs/SPEC-UI/001-SPEC-UI.md §6.11, tab 1).
	//
	// Every filter is URL state (§8.4.2), and the URL is what the request is built from: the table is never
	// filtered in the browser, because §8.4.1 forbids it and one page of rows is not the data set.
	//
	// The filter bar and the purge dialog are their own components; this file owns only the data, the states,
	// and the pagination that tie them to the URL.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import LogsFilters from '$lib/components/LogsFilters.svelte';
	import PurgeLogsDialog from '$lib/components/PurgeLogsDialog.svelte';
	import RequestLogDrawer from '$lib/components/RequestLogDrawer.svelte';
	import RequestLogTable from '$lib/components/RequestLogTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listRequestLogs, purgeRequestLogs } from '$lib/api/log';
	import type { LogRecord } from '$lib/schemas/log';
	import {
		logFiltersApplied,
		nextLogSearch,
		parseLogSearch,
		LOGS_PAGE_SIZE,
		type LogSearch
	} from '$lib/schemas/log-search';
	import { logQuery } from '$lib/schemas/log-view';

	let records = $state<LogRecord[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let opened = $state<string | null>(null);

	let purgeOpen = $state(false);
	let purgeBusy = $state(false);
	let purgeResult = $state<string | null>(null);

	const search = $derived(parseLogSearch(page.url.searchParams));
	const lastPage = $derived(Math.max(1, Math.ceil(total / LOGS_PAGE_SIZE)));
	const filtered = $derived(logFiltersApplied(search));

	// Reloads whenever the URL changes, which is every filter on this tab.
	$effect(() => {
		const filters = search;
		untrack(() => void load(filters));
	});

	async function load(filters: LogSearch): Promise<void> {
		loading = true;
		const result = await listRequestLogs(logQuery(filters, new Date()));
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		records = result.data.data;
		total = result.data.meta.total;
	}

	function navigate(params: URLSearchParams): void {
		const queryString = params.toString();
		void goto(resolve(queryString === '' ? '/logs' : `/logs?${queryString}`), { keepFocus: true });
	}

	function change(key: string, value: string): void {
		navigate(nextLogSearch(page.url.searchParams, key, value));
	}

	function applyFilters(values: { endpointId: string; model: string; query: string }): void {
		const fields = [
			['endpoint_id', values.endpointId],
			['model', values.model],
			['q', values.query]
		] as const;

		let next = page.url.searchParams;
		for (const [key, value] of fields) next = nextLogSearch(next, key, value.trim());

		navigate(next);
	}

	function clearFilters(): void {
		let next = page.url.searchParams;
		for (const key of ['status', 'endpoint_id', 'model', 'q']) next = nextLogSearch(next, key, '');

		navigate(next);
	}

	async function confirmPurge(): Promise<void> {
		purgeBusy = true;
		const result = await purgeRequestLogs();
		purgeBusy = false;

		if (!result.ok) {
			purgeResult = result.error.message;
			return;
		}

		const deleted = result.data.deleted;
		purgeOpen = false;
		purgeResult =
			deleted === 1
				? 'Purged 1 request older than the retention window.'
				: `Purged ${deleted} requests older than the retention window.`;
		await load(search);
	}
</script>

<div class="flex flex-col gap-5">
	<LogsFilters
		{search}
		onchange={change}
		onapply={applyFilters}
		onclear={clearFilters}
		onpurge={() => {
			purgeResult = null;
			purgeOpen = true;
		}}
	/>

	{#if purgeResult}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{purgeResult}</p>
	{/if}

	{#if search.notices.length > 0}
		<div
			role="status"
			aria-live="polite"
			class="flex flex-col gap-1 rounded-[var(--radius-md)] bg-[var(--color-surface-2)] px-3 py-2 text-sm"
		>
			{#each search.notices as notice (notice)}
				<p>{notice}</p>
			{/each}
		</div>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading requests" />
	{:else if error}
		<StateMessage kind="error" title="Requests could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(search)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if records.length === 0 && filtered}
		<StateMessage
			kind="empty"
			title="No requests match these filters"
			description="The period may still hold requests that these filters exclude. Clear the filters to see what it holds."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else if records.length === 0}
		<StateMessage
			kind="empty"
			title="No requests in this window"
			description="Nothing was routed in the selected period. The gateway writes a row here once a client sends a request with a gateway key, so the likely cause is that no client has called it yet."
		>
			{#snippet action()}
				{#if search.period !== '60d'}
					<button type="button" class="underline" onclick={() => change('period', '60d')}
						>Look back 60 days</button
					>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<RequestLogTable {records} onopen={(record) => (opened = record.request_id)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page <= 1}
				onclick={() => change('page', String(search.page - 1))}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">
				Page {search.page} of {lastPage}, {total}
				{total === 1 ? 'request' : 'requests'}
			</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page >= lastPage}
				onclick={() => change('page', String(search.page + 1))}>Next</button
			>
		</div>
	{/if}
</div>

<RequestLogDrawer requestId={opened} onclose={() => (opened = null)} />

<PurgeLogsDialog
	open={purgeOpen}
	busy={purgeBusy}
	result={purgeResult}
	onclose={() => {
		purgeOpen = false;
		purgeResult = null;
	}}
	onconfirm={confirmPurge}
/>
