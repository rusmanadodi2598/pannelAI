<script lang="ts">
	// The facts list on a custom node's detail card (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// It is its own component because the card around it owns the actions and the dialogs, and the facts
	// are the read-only half: what the node is addressed by, where the gateway sends it, what it speaks,
	// and when it last changed. Every value comes from the node's own route, which is the only read that
	// carries the prefix and the api type (the provider read synthesizes an entry without them).
	import { formatTimestamp } from '$lib/utils/time';
	import type { ProviderNode } from '$lib/schemas/provider-node';

	let { node }: { node: ProviderNode } = $props();
</script>

<dl class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Model prefix</dt>
		<dd class="text-sm">
			{node.prefix}/model
		</dd>
	</div>
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Base URL</dt>
		<dd class="break-all text-sm">{node.base_url}</dd>
	</div>
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Wire format</dt>
		<dd class="text-sm">{node.format}</dd>
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
