<script lang="ts">
	// Provider detail (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The screen answers three questions in the order an operator asks them: what this provider is (facts),
	// which models it can answer with (catalog), and which accounts the gateway routes through (endpoints).
	// Each is its own component, so the page's own job is the load, the heading, and the section titles.
	//
	// §6.3 also places the OAuth section, the custom-model editor, and the alias table on this screen. All
	// three are U2, which is why nothing here writes.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import ModelCatalogList from '$lib/components/ModelCatalogList.svelte';
	import ProviderEndpoints from '$lib/components/ProviderEndpoints.svelte';
	import ProviderFacts from '$lib/components/ProviderFacts.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getProvider } from '$lib/api/providers';
	import type { ProviderDetail } from '$lib/schemas/provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const providerId = $derived(params.provider_id);

	let provider = $state<ProviderDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Reloads when the route's provider changes, so a hand-edited URL never leaves one provider's facts
	// above another provider's catalog.
	$effect(() => {
		const id = providerId;
		untrack(() => void load(id));
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const result = await getProvider(id);

		if (!result.ok) {
			loading = false;
			error = result.error.message;
			provider = null;
			return;
		}

		loading = false;
		error = null;
		provider = result.data;
	}
</script>

<section class="flex flex-col gap-6">
	<div class="flex flex-col gap-1">
		<a href={resolve('/providers')} class="w-fit text-sm underline">Provider</a>
		<h1 class="text-lg font-semibold tracking-tight">
			{provider?.name ?? providerId}
		</h1>
		{#if provider}
			<p class="text-sm text-[var(--color-text-muted)]">{provider.id}</p>
		{/if}
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the registry entry" />
	{:else if error}
		<StateMessage kind="error" title="This provider could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId)}>Try again</button>
				<a href={resolve('/providers')} class="ms-3 underline">Back to the registry</a>
			{/snippet}
		</StateMessage>
	{:else if provider}
		<ProviderFacts {provider} />

		<div class="flex flex-col gap-3">
			<h2 class="text-base font-medium">Model catalog</h2>
			<ModelCatalogList {providerId} />
		</div>

		<div class="flex flex-col gap-3">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-base font-medium">Endpoints</h2>
				<a
					href={resolve(`/endpoint-keys?provider=${encodeURIComponent(provider.id)}`)}
					class="min-h-11 content-center underline">Add an endpoint</a
				>
			</div>
			<ProviderEndpoints providerId={provider.id} />
		</div>
	{/if}
</section>
