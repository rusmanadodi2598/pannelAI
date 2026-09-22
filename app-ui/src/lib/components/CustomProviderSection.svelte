<script lang="ts">
	// The custom provider section of the Provider list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// A custom provider is a base URL the embedded registry does not ship, created through
	// docs/SPEC-API/001-SPEC-API.md §7.4. The gateway synthesizes a provider entry from it, so a node
	// appears in the registry table above and routes like any other provider; what the table cannot show
	// is what the node *is*, and that is this section's job: the prefix its models are addressed by, the
	// base URL, and the endpoint the gateway will call.
	//
	// The list is read whole rather than paged, because §7.4 returns every node: a node set is
	// hand-configured, so its bound is the operator's own count. It is also independent of the category
	// filter above, which narrows the registry and not this set.
	//
	// Write actions are not repeated here. Edit, Test, and Delete live on the node's own detail screen,
	// which is where the reference puts them and where the node's other facts are read.
	import { resolve } from '$app/paths';
	import CustomProviderDialog from '$lib/components/CustomProviderDialog.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import {
		nodeEndpointLabel,
		nodeEndpointUrl,
		type NodeType,
		type ProviderNode
	} from '$lib/schemas/provider-node';

	let {
		nodes,
		loading,
		error,
		onreload
	}: {
		nodes: ProviderNode[];
		loading: boolean;
		error: string | null;
		/** Re-reads the node set. The screen's own refresh control calls this beside the registry read. */
		onreload: () => Promise<void>;
	} = $props();

	let target = $state<ProviderNode | 'new' | null>(null);
	let type = $state<NodeType>('openai-compatible');

	function add(kind: NodeType): void {
		type = kind;
		target = 'new';
	}

	const sorted = $derived([...nodes].sort((left, right) => left.name.localeCompare(right.name)));
</script>

<section class="flex flex-col gap-3">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="flex flex-col gap-1">
			<h2 class="text-base font-medium">Custom provider</h2>
			<p class="text-sm text-[var(--color-text-muted)]">
				A base URL the registry does not carry, reached through the OpenAI or Anthropic shape. A
				node's models are addressed as <span class="font-medium">prefix/model</span>.
			</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
				onclick={() => add('anthropic-compatible')}>Add Anthropic Compatible</button
			>
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
				onclick={() => add('openai-compatible')}>Add OpenAI Compatible</button
			>
		</div>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the custom providers" />
	{:else if error}
		<StateMessage kind="error" title="The custom providers could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={onreload}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if sorted.length === 0}
		<StateMessage
			kind="empty"
			title="No custom provider yet"
			description="Use the buttons above to add a base URL the registry does not carry. It becomes routable as soon as a model is declared under its prefix and an endpoint holds a key."
		/>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each sorted as node (node.id)}
				<li
					class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 py-2"
				>
					<div class="flex flex-col">
						<a
							href={resolve('/providers/[provider_id]', { provider_id: node.id })}
							class="font-medium underline">{node.name}</a
						>
						<span class="text-sm text-[var(--color-text-muted)]">
							{node.prefix}/model, {nodeEndpointLabel(node)}
						</span>
					</div>
					<span class="text-sm break-all text-[var(--color-text-muted)]"
						>{nodeEndpointUrl(node)}</span
					>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<CustomProviderDialog {target} {type} onsaved={onreload} onclose={() => (target = null)} />
