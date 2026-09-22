<script lang="ts">
	// The Usage Records filter bar (docs/SPEC-UI/001-SPEC-UI.md §6.5, tab 2).
	//
	// The four selects commit on change, because a select has no half-typed state to protect. The three
	// free-text filters are drafts until the operator submits them, so a half-typed model string does not
	// fire a request per keystroke.
	//
	// The component does not own the applied values: they come from the URL, and `synced` records which
	// applied state the boxes were last filled from, so a back or forward navigation moves the boxes as well
	// as the table while typing is never overwritten by the mirror.
	//
	// The provider and gateway-key selects read their own vocabularies, which is the one place on this
	// screen that does: neither read builds the request, and the bar is what needs the names (draft 014 F3).
	// A registry that cannot be read leaves the select offering the value the URL carries and a line saying
	// why, rather than silently showing "Any provider" over a filtered table.
	import { onMount } from 'svelte';
	import { listGatewayKeys } from '$lib/api/gateway-keys';
	import { listProviders } from '$lib/api/providers';
	import { REQUEST_STATUSES, REQUEST_STATUS_LABELS } from '$lib/schemas/primitives';
	import { USAGE_PERIODS, USAGE_PERIOD_LABELS } from '$lib/schemas/usage';
	import { usageFiltersApplied, type UsageSearch } from '$lib/schemas/usage-search';

	type Props = {
		search: UsageSearch;
		/** Commits one filter, for the controls that have nothing to draft. */
		onchange: (key: string, value: string) => void;
		/** Commits the three text filters together, so one submit is one navigation. */
		onapply: (values: { endpointId: string; model: string; query: string }) => void;
		onclear: () => void;
	};

	type Option = { id: string; name: string };

	let { search, onchange, onapply, onclear }: Props = $props();

	let drafts = $state({ endpoint: '', model: '', query: '' });
	let synced = $state('');
	let providers = $state<Option[]>([]);
	let gatewayKeys = $state<Option[]>([]);
	let providersNotice = $state<string | null>(null);
	let keysNotice = $state<string | null>(null);

	$effect(() => {
		const key = `${search.endpointId}\n${search.model}\n${search.query}`;
		if (key === synced) return;

		synced = key;
		drafts = { endpoint: search.endpointId, model: search.model, query: search.query };
	});

	const filtered = $derived(usageFiltersApplied(search));

	/**
	 * The options a select offers, with the current value kept in the list when the read did not carry it.
	 *
	 * A select that fell back to its empty option while the URL filtered by that id would state something
	 * false about the table below it. The id is shown as its own name, which is what the breakdown table
	 * does for an id the registry cannot resolve.
	 */
	function optionsWith(list: Option[], current: string): Option[] {
		if (current === '' || list.some((option) => option.id === current)) return list;
		return [{ id: current, name: current }, ...list];
	}

	const providerOptions = $derived(optionsWith(providers, search.providerId));
	const keyOptions = $derived(optionsWith(gatewayKeys, search.gatewayKeyId));

	async function loadOptions(): Promise<void> {
		const [providerResult, keyResult] = await Promise.all([
			listProviders({ per_page: 100 }),
			listGatewayKeys({ per_page: 100 })
		]);

		if (providerResult.ok) {
			providers = providerResult.data.data.map((row) => ({ id: row.id, name: row.name }));
		} else {
			providersNotice = `The provider list could not be read (${providerResult.error.message}), so this filter lists only the id the URL carries.`;
		}

		if (keyResult.ok) {
			gatewayKeys = keyResult.data.data.map((row) => ({ id: row.id, name: row.name }));
		} else {
			keysNotice = `The gateway key list could not be read (${keyResult.error.message}), so this filter lists only the id the URL carries.`;
		}
	}

	onMount(() => {
		void loadOptions();
	});

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<form
	class="flex flex-wrap items-end gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		onapply({ endpointId: drafts.endpoint, model: drafts.model, query: drafts.query });
	}}
>
	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Period</span>
		<select
			value={search.period}
			onchange={(event) => onchange('period', event.currentTarget.value)}
			class={fieldClass}
		>
			{#each USAGE_PERIODS as period (period)}
				<option value={period}>{USAGE_PERIOD_LABELS[period]}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Status</span>
		<select
			value={search.status}
			onchange={(event) => onchange('status', event.currentTarget.value)}
			class={fieldClass}
		>
			<option value="">Any status</option>
			{#each REQUEST_STATUSES as status (status)}
				<option value={status}>{REQUEST_STATUS_LABELS[status]}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Provider</span>
		<select
			value={search.providerId}
			onchange={(event) => onchange('provider_id', event.currentTarget.value)}
			class={fieldClass}
		>
			<option value="">Any provider</option>
			{#each providerOptions as option (option.id)}
				<option value={option.id}>{option.name}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Gateway key</span>
		<select
			value={search.gatewayKeyId}
			onchange={(event) => onchange('gateway_key_id', event.currentTarget.value)}
			class={fieldClass}
		>
			<option value="">Any gateway key</option>
			{#each keyOptions as option (option.id)}
				<option value={option.id}>{option.name}</option>
			{/each}
		</select>
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Endpoint id</span>
		<input bind:value={drafts.endpoint} placeholder="ep_..." class={`w-48 ${fieldClass}`} />
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Model</span>
		<input bind:value={drafts.model} placeholder="gpt-4o" class={`w-40 ${fieldClass}`} />
	</label>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Search</span>
		<input
			type="search"
			bind:value={drafts.query}
			placeholder="Request id, error code"
			class={`w-48 ${fieldClass}`}
		/>
	</label>

	<button
		type="submit"
		class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
		>Apply filters</button
	>

	{#if filtered}
		<button type="button" class="min-h-11 underline" onclick={onclear}>Clear filters</button>
	{/if}
</form>

{#if providersNotice !== null || keysNotice !== null}
	<div role="status" aria-live="polite" class="flex flex-col gap-1 text-sm">
		{#if providersNotice !== null}
			<p class="text-[var(--color-text-muted)]">{providersNotice}</p>
		{/if}
		{#if keysNotice !== null}
			<p class="text-[var(--color-text-muted)]">{keysNotice}</p>
		{/if}
	</div>
{/if}
