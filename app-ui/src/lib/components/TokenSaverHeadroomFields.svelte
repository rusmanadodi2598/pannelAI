<script lang="ts">
	// The Headroom group's fields (docs/SPEC-UI/001-SPEC-UI.md §6.7, section 2).
	//
	// Its own file because this group carries the most copy on the screen: the service is external, the
	// call has a 5 second timeout and fails open, and the gateway does not manage the process. §6.7 asks
	// for all three to be stated, and a reader checking that they are stated should not have to find them
	// among two other groups.
	import { headroomUrlWarning } from '$lib/schemas/token-saver-form';
	import type { TokenSaver } from '$lib/schemas/token-saver';

	type Props = { value: TokenSaver['headroom'] };

	let { value = $bindable() }: Props = $props();

	const warning = $derived(headroomUrlWarning(value));
</script>

<label class="flex items-start gap-3 text-sm">
	<input type="checkbox" class="mt-1 size-4" bind:checked={value.enabled} />
	<span>
		Compress through Headroom
		<span class="block text-xs text-[var(--color-text-muted)]">
			Called with a 5 second timeout. A failure or a timeout fails open: the request continues
			uncompressed. The gateway does not start or stop the service.
		</span>
	</span>
</label>

<div class="flex flex-col gap-1 text-sm">
	<label for="headroom-url">Headroom URL</label>
	<input
		id="headroom-url"
		type="text"
		bind:value={value.url}
		placeholder="http://localhost:8787"
		aria-describedby="headroom-url-help"
		class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
	/>
	<span id="headroom-url-help" class="text-xs text-[var(--color-text-muted)]">
		An absolute http or https URL. Kept even while the group is off.
	</span>
</div>

<label class="flex items-start gap-3 text-sm">
	<input type="checkbox" class="mt-1 size-4" bind:checked={value.compress_user_messages} />
	<span>
		Compress user messages too
		<span class="block text-xs text-[var(--color-text-muted)]">
			Off leaves the messages a person wrote out of the pass.
		</span>
	</span>
</label>

{#if warning}
	<p class="text-xs text-[var(--color-warn)]">{warning}</p>
{/if}
