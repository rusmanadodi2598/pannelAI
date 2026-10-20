<script lang="ts">
	// The band a per-section write ends with (docs/SPEC-UI/001-SPEC-UI.md §6.7, §6.13): one commit, the
	// unsaved marker, and the outcome line. Both sections make the save per group, so each group needs its
	// own answers, and four editors share the rule: three settings tabs and the token saver's section shell.
	// The band takes the answers rather than holding them, so no caller can drift from the others.
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
