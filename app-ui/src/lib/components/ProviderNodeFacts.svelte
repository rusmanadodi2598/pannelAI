<script lang="ts">
	// The custom-specific facts on a node's card. The common provider facts are rendered by ProviderFacts
	// above it; this list adds the prefix, api type, and timestamps the synthesized provider entry does
	// not carry.
	import { formatTimestamp } from '$lib/utils/time';
	import { NODE_API_TYPE_LABELS, type ProviderNode } from '$lib/schemas/provider-node';

	let { node }: { node: ProviderNode } = $props();

	function apiTypeLabel(value: string | undefined): string {
		if (value === undefined) return 'Not applicable';
		return value === 'chat' || value === 'responses' ? NODE_API_TYPE_LABELS[value] : value;
	}
</script>

<dl class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Model prefix</dt>
		<dd class="text-sm">
			{node.prefix}/model
		</dd>
	</div>
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">API type</dt>
		<dd class="text-sm">
			{apiTypeLabel(node.api_type)}
		</dd>
	</div>
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Created</dt>
		<dd class="text-sm">{formatTimestamp(node.created_at)}</dd>
	</div>
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Last changed</dt>
		<dd class="text-sm">{formatTimestamp(node.updated_at)}</dd>
	</div>
</dl>
