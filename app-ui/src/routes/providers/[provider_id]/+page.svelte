<script lang="ts">
	// Provider detail (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The screen has two shapes, and which one it renders is decided by the id's prefix, which is the public
	// contract for a node (§7.4) and the only signal the panel has, because the provider response carries no
	// `custom` flag.
	//
	// A registry provider gets the five questions an operator asks about it, in the order they ask them:
	// what it is (facts), which models it can answer with (catalog), which of its models are turned off
	// (disabled), which models it declares itself (custom), and what the gateway routes through it (its
	// connections), followed by the proxy binding and the OAuth section for an OAuth provider.
	//
	// A custom node gets the reference's own page instead (`providers/[id]/page.js:1447-1819`), because
	// none of those five questions is the registry's to answer for it: the node's facts are its own details
	// card, its models are exactly the ones declared for it (there is no registry catalog behind it), and
	// the connection that carries its credential is the first thing to set up. So the details card leads,
	// the connections follow, and the models come after, each in the reference's shape. The registry facts
	// table, the catalog, and the disabled-model section are not rendered for a node: they describe the
	// embedded registry, and a node is not in it.
	//
	// The key dialog belongs to the page rather than to the Connections section, because two places open
	// it: the section's own button and the node's details card. A successful add has to reach the table
	// inside the section, so the page bumps `endpointToken` and hands the section the notice.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import AddProviderKeysDialog from '$lib/components/AddProviderKeysDialog.svelte';
	import CustomProviderCard from '$lib/components/CustomProviderCard.svelte';
	import ModelCatalogList from '$lib/components/ModelCatalogList.svelte';
	import ProviderConnectionsSection from '$lib/components/ProviderConnectionsSection.svelte';
	import ProviderCustomModels from '$lib/components/ProviderCustomModels.svelte';
	import ProviderDisabledModels from '$lib/components/ProviderDisabledModels.svelte';
	import ProviderFacts from '$lib/components/ProviderFacts.svelte';
	import ProviderOAuth from '$lib/components/ProviderOAuth.svelte';
	import ProviderProxyCard from '$lib/components/ProviderProxyCard.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getProvider } from '$lib/api/providers';
	import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import { isNodeId } from '$lib/schemas/provider-node';
	import { REQUIRES_KEY_AUTH_TYPES } from '$lib/schemas/endpoint-write';
	import type { ProviderDetail } from '$lib/schemas/provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const providerId = $derived(params.provider_id);

	// Read once per id, and the node card's own read is separate: a node's prefix and api type are not in
	// the provider response, so the card reads §7.4's route for them and reports the prefix back, which is
	// what the models section addresses its rows with.
	const custom = $derived(isNodeId(providerId));

	let provider = $state<ProviderDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// The disabled set is global, so it is read once rather than per provider. A node does not read it: the
	// set narrows the registry catalog, and a node's screen has no catalog to narrow.
	const disabled = createModelDisabledStore();
	let catalogToken = $state(0);

	// The Connections section's own revision, bumped by the key dialog and by nothing else: a key added here
	// has to appear in the table below without a reload of the whole page.
	let endpointToken = $state(0);
	let addingKey = $state(false);
	let keyNotice = $state<string | null>(null);
	let nodePrefix = $state('');

	// A key is only meaningful where the provider's own auth type takes one. For an OAuth provider the
	// dialog would ask for a credential the provider does not use, and the path that fits is the endpoint
	// form, which carries the auth type with it.
	const offersKeys = $derived(provider !== null && REQUIRES_KEY_AUTH_TYPES.has(provider.auth_type));

	// Reloads when the route's provider changes, so a hand-edited URL never leaves one provider's facts
	// above another provider's catalog.
	$effect(() => {
		const id = providerId;
		untrack(() => void load(id));
	});

	$effect(() => {
		const node = custom;
		untrack(() => {
			if (!node) void disabled.load();
		});
	});

	function bumpCatalog(): void {
		catalogToken += 1;
	}

	function openKeyDialog(): void {
		keyNotice = null;
		addingKey = true;
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
		{#if custom}
			<CustomProviderCard
				{providerId}
				onchanged={() => load(providerId)}
				onaddkey={offersKeys ? openKeyDialog : undefined}
				onprefix={(prefix) => (nodePrefix = prefix)}
			/>

			<ProviderConnectionsSection
				{provider}
				token={endpointToken}
				notice={keyNotice}
				onaddkey={openKeyDialog}
			/>

			<div class="flex flex-col gap-3">
				<h2 class="text-base font-medium">Proxy</h2>
				<ProviderProxyCard providerId={provider.id} />
			</div>

			<div class="flex flex-col gap-3">
				<h2 class="text-base font-medium">Available Models</h2>
				<ProviderCustomModels providerId={provider.id} {nodePrefix} onchanged={bumpCatalog} />
			</div>
		{:else}
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

			<ProviderConnectionsSection
				{provider}
				token={endpointToken}
				notice={keyNotice}
				onaddkey={openKeyDialog}
			/>

			<div class="flex flex-col gap-3">
				<h2 class="text-base font-medium">Proxy</h2>
				<ProviderProxyCard providerId={provider.id} />
			</div>

			{#if provider.has_oauth}
				<div class="flex flex-col gap-3">
					<h2 class="text-base font-medium">OAuth</h2>
					<ProviderOAuth providerId={provider.id} />
				</div>
			{/if}
		{/if}

		<AddProviderKeysDialog
			providerId={provider.id}
			providerName={provider.name}
			authType={provider.auth_type}
			open={addingKey}
			onadded={(added) => {
				endpointToken += 1;
				// The Check glyph rides beside this sentence in the Connections section; the string stays
				// text so the screen reader reads one sentence, not a character and a half of decoration.
				keyNotice =
					added.count === 1 && added.label !== null
						? `${added.label} added.`
						: `${added.count} added.`;
			}}
			onclose={() => (addingKey = false)}
		/>
	{/if}
</section>
