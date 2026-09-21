<script lang="ts">
	// The API base in the three forms a client needs it (docs/SPEC-UI/001-SPEC-UI.md §5.2).
	//
	// The header keeps one control and this dialog owns the rest, because a copy control belongs beside
	// the value it copies: that is where a refused clipboard write can still be answered, with the value
	// on screen and selectable. Each tab states one form, and the tabs are the panel's own PanelTabs, so
	// the panel has one tab language rather than two.
	//
	// The address is the browser's own origin. The panel server forwards /api/v1 to the gateway (§3.1),
	// so a client that can reach the panel needs no second address; outside a browser there is no origin
	// to read and the path alone is what the panel itself would use.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import { API_BASE_COPY as copy } from '$lib/strings/api-base';

	type Props = {
		open: boolean;
		onclose: () => void;
	};

	let { open, onclose }: Props = $props();

	const base = $derived(typeof location === 'undefined' ? '/api/v1' : `${location.origin}/api/v1`);

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
</Modal>
