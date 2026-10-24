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
	// A custom node is a provider the gateway synthesizes, so its Overview leads with the same status and
	// fact cards as a registry provider, followed by its node details card. Its Models tab carries only the
	// models declared for it, and Connections carries the endpoint list, proxy, and OAuth when present.
	// The tabs stay in the same order for both shapes.
	//
	// The key dialog belongs to the page rather than to the Connections section, because two places open
	// it: the section's own button and the node's details card. A successful add has to reach the table
	// inside the section, so the page bumps `endpointToken` and hands the section the notice.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import AddProviderKeysDialog from '$lib/components/AddProviderKeysDialog.svelte';
	import CustomProviderCard from '$lib/components/CustomProviderCard.svelte';
	import ProviderConnectionsSection from '$lib/components/ProviderConnectionsSection.svelte';
	import ProviderModelsSection from '$lib/components/ProviderModelsSection.svelte';
	import SectionFold from '$lib/components/SectionFold.svelte';
	import ProviderFacts from '$lib/components/ProviderFacts.svelte';
	import ProviderOAuth from '$lib/components/ProviderOAuth.svelte';
	import ProviderProxyCard from '$lib/components/ProviderProxyCard.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { getProvider } from '$lib/api/providers';
	import { Button } from '$lib/primitives/button/index.js';
	import { isNodeId } from '$lib/schemas/provider-node';
	import {
		REQUIRES_KEY_AUTH_TYPES,
		keyCredentialLabel,
		takesKeyCredential
	} from '$lib/schemas/endpoint-credential';
	import type { ProviderDetail } from '$lib/schemas/provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const providerId = $derived(params.provider_id);

	// A registry row can deep-link to the Connections tab with the create form open (SPEC-UI §6.3).
	const query =
		typeof window === 'undefined'
			? new URLSearchParams()
			: new URLSearchParams(window.location.search);
	const initialTab = query.get('tab') ?? undefined;
	const initialEndpointForm = initialTab === 'connections' && query.get('add') === 'endpoint';

	// Tabs group the detail into three reads an operator switches between, so the page is not one long run of
	// plain text. Both shapes use the same order, so a custom node never moves the operator's reading path.
	const TABS = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'models', label: 'Models' },
		{ id: 'connections', label: 'Connections' }
	];

	// Read once per id, and the node card's own read is separate: a node's prefix and api type are not in
	// the provider response, so the card reads §7.4's route for them and reports the prefix back, which is
	// what the models section addresses its rows with.
	const custom = $derived(isNodeId(providerId));

	let provider = $state<ProviderDetail | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);

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
		<h1 class="text-lg font-semibold tracking-tight break-words">
			{provider?.name ?? providerId}
		</h1>
		{#if provider}
			<p class="break-all text-sm text-[var(--color-text-muted)]">{provider.id}</p>
		{/if}
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the registry entry" />
	{:else if error}
		<StateMessage kind="error" title="This provider could not be loaded" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="underline" onclick={() => load(providerId)}>Try again</Button
				>
				<a href={resolve('/providers')} class="ms-3 inline-flex min-h-11 items-center underline"
					>Back to the registry</a
				>
			{/snippet}
		</StateMessage>
	{:else if provider}
		{@const loaded = provider}
		<PanelTabs
			tabs={TABS}
			label={custom ? 'Custom provider sections' : 'Provider sections'}
			initial={initialTab}
		>
			{#snippet panel(tab)}
				{#if tab === 'overview'}
					{#if custom}
						<div class="flex flex-col gap-4">
							<ProviderFacts provider={loaded} />
							<CustomProviderCard
								{providerId}
								onchanged={() => load(providerId)}
								onaddkey={offersKeys ? openKeyDialog : undefined}
								onprefix={(prefix) => (nodePrefix = prefix)}
							/>
						</div>
					{:else}
						<ProviderFacts provider={loaded} />
					{/if}
				{:else if tab === 'models'}
					<ProviderModelsSection provider={loaded} {providerId} {custom} {nodePrefix} />
				{:else if tab === 'connections'}
					<SectionFold title="Connections" open={initialEndpointForm}>
						<ProviderConnectionsSection
							provider={loaded}
							token={endpointToken}
							notice={keyNotice}
							initialCreating={initialEndpointForm}
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
