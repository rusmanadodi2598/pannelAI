<script lang="ts">
	// The band every settings tab ends with (docs/SPEC-UI/001-SPEC-UI.md §6.13): one write, the unsaved
	// marker §6.13 asks for, and the outcome line. Three tabs share the rule and each owns its own draft,
	// so the band takes the answers rather than holding them.
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { CONTROL_ICONS } from '$lib/icons';
	import type { ApiError } from '$lib/api/errors';

	type Props = {
		saveLabel: string;
		saving: boolean;
		dirty: boolean;
		/** The last thing the write had to say: the gateway's refusal, or the tab's own sentence. */
		message: string | ApiError | null;
		onsave: () => void;
		ondiscard: () => void;
	};

	let { saveLabel, saving, dirty, message, onsave, ondiscard }: Props = $props();

	const SaveIcon = CONTROL_ICONS.save.icon;
</script>

<div class="flex flex-wrap items-center gap-3">
	<Button disabled={saving} aria-busy={saving} onclick={onsave}>
		<SaveIcon class="size-4" aria-hidden="true" />
		{saveLabel}
	</Button>
	{#if dirty}
		<Button variant="ghost" class="underline" onclick={ondiscard}>Discard changes</Button>
		<span class="text-sm font-medium text-[var(--color-text-muted)]">Unsaved changes</span>
	{/if}
	{#if message}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			<ApiErrorText error={message} />
		</p>
	{/if}
</div>
