<script lang="ts">
	// The connected accounts of one provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// One row per OAuth endpoint, with the token state the gateway derives and the one action that changes
	// it. The panel does not re-derive that state: `refresh_state` is the API's own classification, and the
	// only thing the row adds is the difference between a token inside its refresh window and one whose
	// expiry has passed, which the expiry the row carries already tells apart.
	//
	// The refresh is per account rather than a single button for the provider, because the answer names the
	// accounts it moved and the operator should know which one they acted on. It is the same route either
	// way: an id is what the API reads as "this one".
	import { refreshProviderOAuth } from '$lib/api/providers';
	import { oauthTokenState, type OAuthEndpointStatus } from '$lib/schemas/oauth';
	import { formatTimestamp } from '$lib/utils/time';

	let {
		providerId,
		endpoints,
		onrefreshed
	}: {
		providerId: string;
		endpoints: OAuthEndpointStatus[];
		/** Asks the section to re-read the status, because a refresh moves the state this table renders. */
		onrefreshed: () => Promise<void>;
	} = $props();

	let refreshing = $state<string | null>(null);
	let outcome = $state<{ ok: boolean; message: string } | null>(null);

	// The instant the expiry column is read against. Taken once for the render rather than from a ticking
	// clock: a live countdown would be a second clock disagreeing with the gateway's own classification.
	const now = Date.now();

	async function refresh(endpointId: string): Promise<void> {
		refreshing = endpointId;
		outcome = null;
		const result = await refreshProviderOAuth(providerId, endpointId);
		refreshing = null;

		if (!result.ok) {
			outcome = { ok: false, message: result.error.message };
			return;
		}

		// The count and the ids are both the gateway's, so the sentence reports what it answered rather than
		// what the panel asked for.
		const { refreshed, endpoint_ids: ids } = result.data;
		outcome = {
			ok: true,
			message: `The gateway refreshed ${refreshed} ${refreshed === 1 ? 'account' : 'accounts'}: ${
				ids.length === 0 ? 'none' : ids.join(', ')
			}.`
		};
		await onrefreshed();
	}
</script>

<div class="flex flex-col gap-3">
	<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
		<table class="w-full min-w-[52rem] border-collapse text-sm">
			<caption class="sr-only">Connected OAuth accounts for this provider</caption>
			<thead class="bg-[var(--color-surface-2)] text-left">
				<tr>
					<th scope="col" class="px-3 py-2 font-medium">Account</th>
					<th scope="col" class="px-3 py-2 font-medium">Endpoint</th>
					<th scope="col" class="px-3 py-2 font-medium">Expires</th>
					<th scope="col" class="px-3 py-2 font-medium">Last refresh</th>
					<th scope="col" class="px-3 py-2 font-medium">Token state</th>
					<th scope="col" class="px-3 py-2 font-medium">
						<span class="sr-only">Actions</span>
					</th>
				</tr>
			</thead>
			<tbody>
				{#each endpoints as endpoint (endpoint.endpoint_id)}
					<tr class="border-t border-[var(--color-border)]">
						<td class="px-3 py-2">
							<span class="font-medium">{endpoint.label || endpoint.endpoint_id}</span>
							<br />
							<span class="text-[var(--color-text-muted)]">{endpoint.endpoint_id}</span>
						</td>
						<td class="px-3 py-2">{endpoint.status}</td>
						<td class="px-3 py-2">
							{endpoint.expires_at ? formatTimestamp(endpoint.expires_at) : 'No expiry known'}
						</td>
						<td class="px-3 py-2">
							{endpoint.last_refresh_at
								? formatTimestamp(endpoint.last_refresh_at)
								: 'No refresh recorded'}
						</td>
						<td class="px-3 py-2">
							{oauthTokenState(endpoint.refresh_state, endpoint.expires_at, now)}
						</td>
						<td class="px-3 py-2 text-end">
							<button
								type="button"
								class="min-h-11 underline disabled:opacity-50"
								disabled={refreshing !== null}
								onclick={() => void refresh(endpoint.endpoint_id)}
							>
								{refreshing === endpoint.endpoint_id ? 'Refreshing' : 'Refresh'}
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if outcome}
		<p
			class="text-sm"
			role={outcome.ok ? 'status' : 'alert'}
			class:text-[var(--color-danger)]={!outcome.ok}
		>
			{outcome.message}
		</p>
	{/if}
</div>
