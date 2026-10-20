<script lang="ts">
	// The provider registry table (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Columns follow the operator's question here: can the gateway route to this provider at all, and does it
	// have a working account. The name carries the id beneath it because the registry has both and an
	// operator matching a log line needs the id, not the display name. A provider with no endpoint links
	// straight to the create form for that provider, which is the one action its row can offer.
	//
	// The two row actions are links rather than buttons, because both navigate: the glyph rides beside the
	// label (CONTROL_ICONS.open, CONTROL_ICONS.add), the way the reference renders its own provider rows'
	// actions, and the label stays because a link's name is what the operator reads. An action link is a
	// control (DESIGN.md §3.5), so both go through the button: it renders an anchor when given `href`,
	// which keeps the role and the navigation and takes the four states off the call site.
	import { resolve } from '$app/paths';
	import { Button } from '$lib/primitives/button/index.js';
	import { CONTROL_ICONS } from '$lib/icons';
	import { AUTH_TYPE_LABELS } from '$lib/schemas/endpoint';
	import { statusSummaryText, type Provider } from '$lib/schemas/provider';

	const OpenIcon = CONTROL_ICONS.open.icon;
	const AddIcon = CONTROL_ICONS.add.icon;

	let { providers }: { providers: Provider[] } = $props();
</script>

<div
	class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
>
	<table class="mobile-key-value-table w-full border-collapse text-sm">
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
					<td data-label="Provider" class="px-3 py-2">
						<span class="font-medium">{provider.name}</span>
						<br />
						<span class="text-[var(--color-text-muted)]">{provider.id}</span>
					</td>
					<td data-label="Category" class="px-3 py-2">{provider.category}</td>
					<td data-label="Auth" class="px-3 py-2"
						>{AUTH_TYPE_LABELS[provider.auth_type] ?? provider.auth_type}</td
					>
					<td data-label="Endpoints" class="px-3 py-2 tabular-nums">{provider.endpoint_count}</td>
					<td data-label="Status" class="px-3 py-2">{statusSummaryText(provider.status_summary)}</td
					>
					<td data-label="Actions" class="px-3 py-2">
						<div class="flex flex-wrap items-center gap-2">
							<Button
								href={resolve('/providers/[provider_id]', { provider_id: provider.id })}
								variant="ghost"
								class="gap-1 px-2 underline"
							>
								<OpenIcon class="size-4" aria-hidden="true" />
								Open
							</Button>
							{#if provider.endpoint_count === 0}
								<Button
									href={resolve('/providers/[provider_id]', { provider_id: provider.id })}
									variant="ghost"
									class="gap-1 px-2 underline"
								>
									<AddIcon class="size-4" aria-hidden="true" />
									Add endpoint
								</Button>
							{/if}
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
