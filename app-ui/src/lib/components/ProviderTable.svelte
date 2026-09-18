<script lang="ts">
	// The provider registry table (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Columns follow the operator's question here: can the gateway route to this provider at all, and does it
	// have a working account. The name carries the id beneath it because the registry has both and an
	// operator matching a log line needs the id, not the display name. A provider with no endpoint links
	// straight to the create form for that provider, which is the one action its row can offer.
	import { resolve } from '$app/paths';
	import { AUTH_TYPE_LABELS } from '$lib/schemas/endpoint';
	import { statusSummaryText, type Provider } from '$lib/schemas/provider';

	let { providers }: { providers: Provider[] } = $props();
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[48rem] border-collapse text-sm">
		<caption class="sr-only">Provider registry, as the gateway sees it</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Provider</th>
				<th scope="col" class="px-3 py-2 font-medium">Category</th>
				<th scope="col" class="px-3 py-2 font-medium">Auth</th>
				<th scope="col" class="px-3 py-2 font-medium">Endpoints</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each providers as provider (provider.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">
						<span class="font-medium">{provider.name}</span>
						<br />
						<span class="text-[var(--color-text-muted)]">{provider.id}</span>
					</td>
					<td class="px-3 py-2">{provider.category}</td>
					<td class="px-3 py-2">{AUTH_TYPE_LABELS[provider.auth_type] ?? provider.auth_type}</td>
					<td class="px-3 py-2 tabular-nums">{provider.endpoint_count}</td>
					<td class="px-3 py-2">{statusSummaryText(provider.status_summary)}</td>
					<td class="px-3 py-2">
						<div class="flex flex-wrap gap-2">
							<a
								href={resolve('/providers/[provider_id]', { provider_id: provider.id })}
								class="min-h-11 content-center underline">Open</a
							>
							{#if provider.endpoint_count === 0}
								<a
									href={`${resolve('/endpoint-keys')}?provider=${encodeURIComponent(provider.id)}`}
									class="min-h-11 content-center underline">Add endpoint</a
								>
							{/if}
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
