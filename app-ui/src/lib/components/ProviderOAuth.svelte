<script lang="ts">
	// The OAuth section of the provider detail (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section appears for a provider whose registry entry says it has OAuth (`has_oauth`), which is the
	// API's own answer to that question, and the flow the status route reports decides what it offers: the
	// panel can start `code` and nothing else, so the other three flows get their reason instead of a
	// control that cannot act (R-26).
	//
	// It reads the status once per visit, as §6.3 asks, and re-reads it after a refresh or after the
	// callback returns, because both move the state it renders. The callback's outcome arrives in this
	// page's own query, so the section reads it, renders it as the gateway's report, and drops those keys
	// from the address so a reload does not announce a past result.
	//
	// The panel does not navigate to the authorize URL on its own: it renders it as a link. A scripted
	// navigation to a third-party address is a redirect the operator did not ask for, and a link shows the
	// host before it is followed.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import ProviderOAuthAccounts from '$lib/components/ProviderOAuthAccounts.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { providerOAuthStatus, startProviderOAuth } from '$lib/api/providers';
	import {
		OAUTH_RETURN_KEYS,
		hasOAuthReturn,
		oauthFlowCopy,
		oauthFlowStartable,
		parseOAuthReturn,
		type OAuthReturn,
		type OAuthStatus
	} from '$lib/schemas/oauth';

	let { providerId }: { providerId: string } = $props();

	let status = $state<OAuthStatus | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// The one-time outcome of a round trip, held in memory so dropping it from the address does not drop the
	// message. Cleared when the operator starts another attempt, because that attempt has no result yet.
	let returned = $state<OAuthReturn | null>(null);

	let starting = $state(false);
	let startError = $state<string | null>(null);
	let authorizeUrl = $state<string | null>(null);

	// One read per visit. The query is read untracked so that dropping the return keys from the address does
	// not re-run this: the return matters on the visit that carries it, which is the visit this effect runs
	// for.
	$effect(() => {
		const id = providerId;
		untrack(() => {
			const params = page.url.searchParams;
			void boot(id, hasOAuthReturn(params) ? params : null);
		});
	});

	// `carried` is the visit's query when it holds a key of ours, and null when it does not. A value the
	// panel does not know leaves `returned` null and is still dropped from the address: the keys are the
	// panel's, so it cleans up after itself whether or not it understood what it found.
	async function boot(id: string, carried: URLSearchParams | null): Promise<void> {
		returned = carried === null ? null : parseOAuthReturn(carried);
		await load(id);
		if (carried !== null) cleanReturn(id);
	}

	// `keep` is the re-read after an action, which already has state on screen: blanking the section to the
	// loading message would unmount the table and, with it, the outcome sentence that asked for the re-read.
	async function load(id: string, keep = false): Promise<void> {
		if (!keep) loading = true;
		const result = await providerOAuthStatus(id);
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			status = null;
			return;
		}

		error = null;
		status = result.data;
	}

	function cleanReturn(id: string): void {
		const params = new SvelteURLSearchParams(page.url.searchParams);
		for (const key of OAUTH_RETURN_KEYS) params.delete(key);

		const query = params.toString();
		const options = { replaceState: true, keepFocus: true };
		// The typed route carries its query inside the route string, so the two shapes are written out rather
		// than concatenated into a value the compiler cannot match to a route.
		if (query === '') {
			void goto(resolve('/providers/[provider_id]', { provider_id: id }), options);
			return;
		}
		void goto(resolve(`/providers/[provider_id]?${query}`, { provider_id: id }), options);
	}

	async function start(): Promise<void> {
		starting = true;
		startError = null;
		returned = null;
		const result = await startProviderOAuth(providerId);
		starting = false;

		if (!result.ok) {
			startError = result.error.message;
			authorizeUrl = null;
			return;
		}

		authorizeUrl = result.data.authorize_url;
	}
</script>

<div class="flex flex-col gap-4">
	{#if returned}
		<p
			class="text-sm"
			role={returned.outcome === 'connected' ? 'status' : 'alert'}
			class:text-[var(--color-danger)]={returned.outcome === 'error'}
		>
			{#if returned.outcome === 'connected'}
				The gateway connected an account{returned.endpointId ? ` (${returned.endpointId})` : ''}.
			{:else}
				The authorization did not complete. The gateway reported: {returned.reason}
			{/if}
		</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading the OAuth state" />
	{:else if error}
		<StateMessage kind="error" title="The OAuth state could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if status}
		<p class="text-sm text-[var(--color-text-muted)]">{oauthFlowCopy(status.flow)}</p>

		{#if oauthFlowStartable(status.flow)}
			<div class="flex flex-wrap items-center gap-3">
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
					disabled={starting}
					onclick={() => void start()}
				>
					{starting ? 'Starting' : 'Start the authorization'}
				</button>

				{#if authorizeUrl}
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the authorize URL is the provider's own absolute URL, never a route of this app -->
					<a href={authorizeUrl} class="min-h-11 content-center underline">
						Open the authorization page
					</a>
				{/if}
			</div>

			<p class="text-sm text-[var(--color-text-muted)]">
				The gateway answers with the provider's own authorize URL. The provider sends the browser
				back to the gateway, which returns it to this page with the outcome.
			</p>

			{#if startError}
				<p class="text-sm text-[var(--color-danger)]" role="alert">{startError}</p>
			{/if}
		{/if}

		{#if status.endpoints.length === 0}
			<StateMessage
				kind="empty"
				title="No account is connected"
				description="No OAuth account is connected for this provider. An account appears here once an authorization completes."
			/>
		{:else}
			<ProviderOAuthAccounts
				{providerId}
				endpoints={status.endpoints}
				onrefreshed={() => load(providerId, true)}
			/>
		{/if}
	{/if}
</div>
