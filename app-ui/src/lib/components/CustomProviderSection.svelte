<script lang="ts">
	// Add controls for the two custom provider types (docs/SPEC-UI/001-SPEC-UI.md §6.3, §7.4).
	//
	// The registry table is the one list of providers, including synthesized custom nodes, so this section
	// only creates one. The dialog posts the node and the page re-reads the registry so the new row appears
	// with every other provider.
	import CustomProviderDialog from '$lib/components/CustomProviderDialog.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { CONTROL_ICONS } from '$lib/icons';
	import type { NodeType } from '$lib/schemas/provider-node';

	const AddIcon = CONTROL_ICONS.add.icon;

	let { onsaved }: { onsaved: () => Promise<void> } = $props();

	let target = $state<'new' | null>(null);
	let type = $state<NodeType>('openai-compatible');

	function add(kind: NodeType): void {
		type = kind;
		target = 'new';
	}
</script>

<section class="flex flex-col gap-2">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div class="flex flex-col gap-1">
			<h2 class="text-base font-medium">Add a custom provider</h2>
			<p class="text-sm text-[var(--color-text-muted)]">
				A base URL the registry lacks. Models are named
				<span class="font-medium">prefix/model</span>. The provider appears in the registry below.
			</p>
		</div>
		<div class="grid w-full grid-cols-1 gap-2 sm:w-auto sm:grid-cols-2">
			<Button variant="outline" class="w-full" onclick={() => add('anthropic-compatible')}>
				<AddIcon class="size-4" aria-hidden="true" />
				Add Anthropic Compatible
			</Button>
			<Button variant="outline" class="w-full" onclick={() => add('openai-compatible')}>
				<AddIcon class="size-4" aria-hidden="true" />
				Add OpenAI Compatible
			</Button>
		</div>
	</div>
</section>

<CustomProviderDialog {target} {type} {onsaved} onclose={() => (target = null)} />
