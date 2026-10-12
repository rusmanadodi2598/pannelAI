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
	// A custom node gets the reference's own page instead (`providers/[id]/page.js`), because
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
	import SectionFold from '$lib/components/SectionFold.svelte';
	import ProviderDisabledModels from '$lib/components/ProviderDisabledModels.svelte';
	import ProviderFacts from '$lib/components/ProviderFacts.svelte';
	import ProviderOAuth from '$lib/components/ProviderOAuth.svelte';
	import ProviderProxyCard from '$lib/components/ProviderProxyCard.svelte';
	import ProviderThinkingControl from '$lib/components/ProviderThinkingControl.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { getProvider } from '$lib/api/providers';
	import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import { createModelTestStore } from '$lib/stores/model-test.svelte';
	import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
	import { isNodeId } from '$lib/schemas/provider-node';
	import {
		REQUIRES_KEY_AUTH_TYPES,
		keyCredentialLabel,
		takesKeyCredential
	} from '$lib/schemas/endpoint-write';
	import type { ProviderDetail } from '$lib/schemas/provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const providerId = $derived(params.provider_id);

	// Tabs group the detail into three reads an operator switches between, so the page is not one long run of
	// plain text. A registry provider and a custom node get the same three tabs with their own contents.
	const REGISTRY_TABS = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'models', label: 'Models' },
		{ id: 'connections', label: 'Connections' }
	];
	const NODE_TABS = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'connections', label: 'Connections' },
		{ id: 'models', label: 'Models' }
	];

	// Read once per id, and the node card's own read is separate: a node's prefix and api type are not in
	// the provider response, so the card reads §7.4's route for them and reports the prefix back, which is
	// what the models section addresses its rows with.
	const custom = $derived(isNodeId(providerId));

	let provider = $state<ProviderDetail | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

	// The disabled set is global, so it is read once rather than per provider. A node does not read it: the
	// set narrows the registry catalog, and a node's screen has no catalog to narrow.
	// One probe store for the screen: a node's declared rows and a registry provider's catalog rows are
	// the same models addressed the same way, so they carry one verdict rather than one per table.
	const modelTests = createModelTestStore();
	const disabled = createModelDisabledStore();
	let catalogToken = $state(0);

	// The reasoning mode is the models section's own setting (SPEC-API §7.14, §7.15): the picker writes it
	// and the two model tables read it for the `(level)` suffix they copy, so the page owns the one store
	// both halves share. It is read for every provider, including one whose picker does not render: a
	// custom node's rows carry levels too, and the tables below still append the suffix.
	const thinking = createProviderThinkingStore();

	// The Connections section's own revision, bumped by the key dialog and by nothing else: a key added here
	// has to appear in the table below without a reload of the whole page.
	let endpointToken = $state(0);
	let addingKey = $state(false);
	let keyNotice = $state<string | null>(null);
	let nodePrefix = $state('');

	// A key is meaningful wherever the provider can carry one (its derived auth type, or a key mode it lists
	// alongside OAuth). Qoder declares `oauth` and `apikey`, so it can be connected through a Personal Access
	// Token pasted into the dialog as well as through the device flow. The same rules the Connections section
	// renders its button by: one home for "does this provider take a key", and one for what the dialog calls
	// the credential it collects.
	const offersKeys = $derived(
		provider !== null && takesKeyCredential(provider.auth_type, provider.auth_modes)
	);

	// The auth type the dialog submits a pasted credential as. A key provider keeps its own; an OAuth provider
	// that also lists a key mode stores the PAT as an `api_key` connection, because the credential the
	// operator typed is a static key, not the OAuth token the device flow would have minted.
	const keyAuthType = $derived(
		provider === null || REQUIRES_KEY_AUTH_TYPES.has(provider.auth_type)
			? (provider?.auth_type ?? 'api_key')
			: 'api_key'
	);

	// The field the operator pastes the credential into, named for what it is: a Personal Access Token for a
	// provider that is OAuth with a key mode too (Qoder), a plain API key otherwise.
	const credentialLabel = $derived(
		provider === null
			? 'API Key'
			: keyCredentialLabel(provider.auth_type, provider.auth_modes, provider.has_oauth)
	);

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

	// Reloads with the provider, like the detail read above: the store answers a mode only for the provider
	// it was last asked about, so a route swap shows no suffix until the new answer lands.
	$effect(() => {
		const id = providerId;
		untrack(() => void thinking.load(id));
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
			error = result.error;
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
		<a
			href={resolve('/providers')}
			class="inline-flex min-h-11 w-fit items-center text-sm underline">Provider</a
		>
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
		<StateMessage kind="error" title="This provider could not be loaded" apiError={error}>
			{#snippet action()}
				<button
					type="button"
					class="inline-flex min-h-11 items-center underline"
					onclick={() => load(providerId)}>Try again</button
				>
				<a href={resolve('/providers')} class="ms-3 inline-flex min-h-11 items-center underline"
					>Back to the registry</a
				>
			{/snippet}
		</StateMessage>
	{:else if provider}
		{@const loaded = provider}
		<PanelTabs
			tabs={custom ? NODE_TABS : REGISTRY_TABS}
			label={custom ? 'Custom provider sections' : 'Provider sections'}
		>
			{#snippet panel(tab)}
				{#if tab === 'overview'}
					{#if custom}
						<CustomProviderCard
							{providerId}
							onchanged={() => load(providerId)}
							onaddkey={offersKeys ? openKeyDialog : undefined}
							onprefix={(prefix) => (nodePrefix = prefix)}
						/>
					{:else}
						<ProviderFacts provider={loaded} />
					{/if}
				{:else if tab === 'models'}
					{#if custom}
						<SectionFold title="Available Models">
							<div class="flex flex-col gap-3">
								<ProviderThinkingControl {providerId} levels={loaded.thinking_levels} {thinking} />
								<ProviderCustomModels
									{providerId}
									{thinking}
									tests={modelTests}
									{nodePrefix}
									onchanged={bumpCatalog}
								/>
							</div>
						</SectionFold>
					{:else}
						<SectionFold title="Model catalog">
							<div class="flex flex-col gap-3">
								<ProviderThinkingControl
									providerId={loaded.id}
									levels={loaded.thinking_levels}
									{thinking}
								/>
								<ModelCatalogList
									tests={modelTests}
									{providerId}
									{disabled}
									{thinking}
									onchanged={bumpCatalog}
									token={catalogToken}
								/>
							</div>
						</SectionFold>

						<SectionFold title="Models this provider cannot route">
							<ProviderDisabledModels {providerId} {disabled} onchanged={bumpCatalog} />
						</SectionFold>

						<SectionFold title="Custom models">
							<ProviderCustomModels
								providerId={loaded.id}
								{thinking}
								tests={modelTests}
								onchanged={bumpCatalog}
							/>
						</SectionFold>
					{/if}
				{:else if tab === 'connections'}
					<SectionFold title="Connections">
						<ProviderConnectionsSection
							provider={loaded}
							token={endpointToken}
							notice={keyNotice}
							onaddkey={openKeyDialog}
						/>
					</SectionFold>

					<SectionFold title="Proxy">
						<ProviderProxyCard {providerId} />
					</SectionFold>

					{#if !custom && loaded.has_oauth}
						<SectionFold title="OAuth">
							<ProviderOAuth {providerId} />
						</SectionFold>
					{/if}
				{/if}
			{/snippet}
		</PanelTabs>

		<AddProviderKeysDialog
			providerId={provider.id}
			providerName={provider.name}
			authType={keyAuthType}
			authHint={provider.auth_hint}
			{credentialLabel}
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
