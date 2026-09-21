<script lang="ts">
	// The API base in the three forms a client needs it (docs/SPEC-UI/001-SPEC-UI.md §5.2).
	//
	// The header keeps one control and this dialog owns the rest, because a copy control belongs beside
	// the value it copies: that is where a refused clipboard write can still be answered, with the value
	// on screen and selectable. Each tab states one form, and the tabs are the panel's own PanelTabs, so
	// the panel has one tab language rather than two.
	//
	// The address is read from the panel server rather than derived from the browser. `location.origin` is
	// the panel, and it is a bind-all address like `http://0.0.0.0:3000` whenever the operator opens the
	// panel that way, which no client can call; the panel server is the component that holds the gateway's
	// address. A read that fails is stated with the gateway's own sentence and a way to ask again, and the
	// dialog never falls back to the browser's origin, because an address that looks right and is not is
	// worse than no address.
	//
	// The three states are `reading`, `failure`, and the answer, in that order of precedence. The address
	// itself is a plain string rather than a nullable one, so the tab snippet takes a `string` and the
	// loaded branch cannot be reached with nothing to show: `reading` and `failure` are what say whether an
	// answer is in hand.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import { fetchApiBase } from '$lib/api/api-base';
	import { API_BASE_COPY as copy } from '$lib/strings/api-base';

	type Props = {
		open: boolean;
		onclose: () => void;
	};

	let { open, onclose }: Props = $props();

	let reading = $state(true);
	let failure = $state<string | null>(null);
	let base = $state('');

	async function load(): Promise<void> {
		reading = true;
		failure = null;

		const result = await fetchApiBase();
		reading = false;

		if (!result.ok) {
			failure = result.error.message;
			base = '';
			return;
		}

		base = result.data.base_url;
	}

	// Read on every open: the address comes from the panel's environment, so a panel restarted with a
	// different target must not be described by the previous answer.
	$effect(() => {
		if (open) void load();
	});

	const TABS = [
		{ id: 'base-url', label: copy.tabs.baseUrl },
		{ id: 'curl', label: copy.tabs.curl },
		{ id: 'openai', label: copy.tabs.openai }
	];
</script>

{#snippet block(value: string, note: string)}
	<div class="flex flex-col gap-2">
		<pre
			class="overflow-x-auto rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-3 py-2 text-xs">{value}</pre>
		<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
			<p class="min-w-0 flex-1 text-xs text-[var(--color-text-muted)]">{note}</p>
			<CopyButton {value} />
		</div>
	</div>
{/snippet}

<Modal title={copy.title} {open} {onclose}>
	{#if open}
		<p class="text-sm text-[var(--color-text-muted)]">{copy.intro}</p>

		{#if reading}
			<p class="mt-4 text-sm text-[var(--color-text-muted)]">{copy.loading}</p>
		{:else if failure}
			<div class="mt-4 flex flex-col items-start gap-2">
				<p class="text-sm text-[var(--color-danger)]" role="alert">{failure}</p>
				<button type="button" class="underline" onclick={load}>{copy.retry}</button>
			</div>
		{:else}
			<PanelTabs tabs={TABS} label={copy.tabsLabel}>
				{#snippet panel(active)}
					{#if active === 'curl'}
						{@render block(copy.curl.command(base), copy.curl.note)}
					{:else if active === 'openai'}
						{@render block(copy.openai.env(base), copy.openai.note)}
					{:else}
						{@render block(base, copy.baseUrl.note)}
					{/if}
				{/snippet}
			</PanelTabs>
		{/if}
	{/if}
</Modal>
