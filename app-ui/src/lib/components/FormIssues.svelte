<script lang="ts">
	// The validation-message list a form shows when the schema refuses a save (docs/SPEC-UI §7.1.8).
	//
	// One component, so every form reports refusals the same way: an assertive region the screen reader
	// announces, and a list where each line is one rule that failed. The messages are the schema's own, which
	// is what keeps the panel and the API from describing the same rule two different ways.
	//
	// Renders nothing when there is nothing to say, so a caller can place it without an `{#if}`.
	let { issues }: { issues: string[] } = $props();
</script>

{#if issues.length > 0}
	<ul
		class="flex flex-col gap-1 rounded-[var(--radius-sm)] border border-[var(--color-danger)] px-3 py-2 text-sm"
		role="alert"
	>
		{#each issues as issue (issue)}
			<li>{issue}</li>
		{/each}
	</ul>
{/if}
