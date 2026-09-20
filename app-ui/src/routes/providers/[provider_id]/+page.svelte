<script lang="ts">
	// Provider detail (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The screen answers five questions about this provider in the order an operator asks them: what it is
	// (facts), which models it can answer with (catalog), which of its models are turned off (disabled),
	// which models it declares itself (custom), and what the gateway routes through it (endpoints). Each is
	// its own component, so the page's own job is the loads, the heading, the section titles, and the one
	// piece of state two sections share.
	//
	// That shared state is the disabled set. Disabling a model removes its catalog row and enabling one
	// brings it back, so the catalog reloads after a write in either section: `catalogToken` is what tells
	// it to. §6.3 also places the OAuth section and the alias table here. OAuth is still U2. The alias table
	// is the sixth section and takes no provider, because the alias set is global.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import ModelCatalogList from '$lib/components/ModelCatalogList.svelte';
	import ProviderAliases from '$lib/components/ProviderAliases.svelte';
	import ProviderCustomModels from '$lib/components/ProviderCustomModels.svelte';
	import ProviderDisabledModels from '$lib/components/ProviderDisabledModels.svelte';
	import ProviderEndpoints from '$lib/components/ProviderEndpoints.svelte';
	import ProviderFacts from '$lib/components/ProviderFacts.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getProvider } from '$lib/api/providers';
	import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import type { ProviderDetail } from '$lib/schemas/provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const providerId = $derived(params.provider_id);

	let provider = $state<ProviderDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// The disabled set is global, so it is read once rather than per provider.
	const disabled = createModelDisabledStore();
	let catalogToken = $state(0);

	// Reloads when the route's provider changes, so a hand-edited URL never leaves one provider's facts
	// above another provider's catalog.
	$effect(() => {
		const id = providerId;
		untrack(() => void load(id));
	});

	$effect(() => {
		untrack(() => void disabled.load());
	});

	function bumpCatalog(): void {
		catalogToken += 1;
	}

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
			<ModelCatalogList {providerId} {disabled} onchanged={bumpCatalog} token={catalogToken} />
		</div>

		<div class="flex flex-col gap-3">
			<h2 class="text-base font-medium">Models this provider cannot route</h2>
			<ProviderDisabledModels {providerId} {disabled} onchanged={bumpCatalog} />
		</div>

		<div class="flex flex-col gap-3">
			<h2 class="text-base font-medium">Custom models</h2>
			<ProviderCustomModels {providerId} onchanged={bumpCatalog} />
		</div>

		<div class="flex flex-col gap-3">
			<h2 class="text-base font-medium">Aliases</h2>
			<ProviderAliases />
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
