<script lang="ts">
	// Settings (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	//
	// Five tabs, one settings group each. Three rules shape the screen:
	//
	//   A tab saves with `PATCH` carrying only its own group, so saving Routing cannot overwrite a
	//   Network value the operator changed in another tab. The per-group patch schemas are what make
	//   that a property of the wire rather than a habit.
	//
	//   Each tab owns its draft, its dirty indicator, and its discard action, so the indicator can only
	//   describe the tab it sits on. A single page-level flag would claim changes on a tab nobody has
	//   open.
	//
	//   After a write the page re-reads the whole document (§8.6.3), so a tab the operator returns to
	//   shows what the gateway stored rather than what the panel last saw.
	//
	// Two tabs are pointers rather than editors. Token Saver and Network each have one editor, on
	// their own screen, and a second editor here is how two views of one configuration drift apart. So
	// each tab states what its screen holds and links to it, which R-24 allows precisely because those
	// routes exist.
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import SettingsSecurityTab from '$lib/components/SettingsSecurityTab.svelte';
	import SettingsRoutingTab from '$lib/components/SettingsRoutingTab.svelte';
	import SettingsLoggingTab from '$lib/components/SettingsLoggingTab.svelte';
	import { fetchSettings } from '$lib/api/settings';
	import type { PanelSettings } from '$lib/schemas/settings';

	const TABS = [
		{ id: 'security', label: 'Security' },
		{ id: 'routing', label: 'Routing' },
		{ id: 'network', label: 'Network' },
		{ id: 'logging', label: 'Logging' },
		{ id: 'token-saver', label: 'Token Saver' }
	];

	let settings = $state<PanelSettings | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await fetchSettings();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		settings = result.data;
	}
</script>

<section class="flex max-w-2xl flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			Panel and gateway configuration. Each tab saves only its own group.
		</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading settings" />
	{:else if error}
		<StateMessage kind="error" title="Settings could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if settings}
		{@const panelSettings = settings}
		<PanelTabs tabs={TABS} label="Settings groups">
			{#snippet panel(id)}
				{#if id === 'security'}
					<SettingsSecurityTab loaded={panelSettings.security} onrefresh={load} />
				{:else if id === 'routing'}
					<SettingsRoutingTab loaded={panelSettings.routing} onrefresh={load} />
				{:else if id === 'network'}
					<div
						class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
					>
						<h2 class="text-sm font-semibold">Network</h2>
						<p class="text-sm text-[var(--color-text-muted)]">
							The outbound proxy settings are global: they decide the path every upstream call
							takes. Their editor sits on Proxy Pools, beside the candidates an operator keeps and
							tests, so this tab does not repeat the form.
						</p>
						<a href={resolve('/proxy-pools')} class="min-h-11 self-start underline">
							Open Proxy Pools
						</a>
					</div>
				{:else if id === 'logging'}
					<SettingsLoggingTab loaded={panelSettings.logging} onrefresh={load} />
				{:else if id === 'token-saver'}
					<div
						class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
					>
						<h2 class="text-sm font-semibold">Token Saver</h2>
						<p class="text-sm text-[var(--color-text-muted)]">
							RTK, Headroom, and Ponytail configure how a request is trimmed before it is sent.
							Their editor is its own screen, so this tab does not repeat the form: two editors for
							one configuration drift apart.
						</p>
						<a href={resolve('/token-saver')} class="min-h-11 self-start underline">
							Open Token Saver
						</a>
					</div>
				{/if}
			{/snippet}
		</PanelTabs>
	{/if}
</section>
