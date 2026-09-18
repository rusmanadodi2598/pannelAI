<script lang="ts">
	// A tablist for the panel's multi-tab screens (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Built here rather than added to `src/lib/primitives/`, because that directory is the component
	// layer's generated output and §10.7 keeps it unedited apart from the one sidebar width token. The
	// markup follows the WAI-ARIA tabs pattern so it is operable by keyboard: arrow keys move between
	// tabs and take focus with them, Home and End jump to the ends, and only the selected tab is in the
	// tab order, which is what lets Tab leave the tablist instead of walking every tab.
	//
	// The selected tab carries the panel's identity motif: a 2px accent rule on the leading edge of the
	// active item (DESIGN.md §6). It is always paired with a weight change and the label, so the active
	// tab is never signalled by colour alone.
	import type { Snippet } from 'svelte';

	type Tab = { id: string; label: string };

	let {
		tabs,
		label,
		panel
	}: {
		tabs: Tab[];
		label: string;
		panel: Snippet<[string]>;
	} = $props();

	// `selected` starts unset and falls back to the first tab, so the active tab follows the tab list if it
	// is ever rendered with a different one, instead of freezing whatever the first render happened to see.
	let selected = $state<string | null>(null);
	const active = $derived(selected ?? tabs[0]?.id ?? '');
	let buttons = $state<HTMLButtonElement[]>([]);

	// Arrow keys move the selection and the focus together, which is the behaviour a tablist is expected
	// to have. Any other key is left alone so Tab and Enter keep working normally.
	function move(event: KeyboardEvent, index: number): void {
		const last = tabs.length - 1;
		let next: number | null = null;

		if (event.key === 'ArrowRight') next = index === last ? 0 : index + 1;
		else if (event.key === 'ArrowLeft') next = index === 0 ? last : index - 1;
		else if (event.key === 'Home') next = 0;
		else if (event.key === 'End') next = last;

		if (next === null) return;

		event.preventDefault();
		selected = tabs[next].id;
		buttons[next]?.focus();
	}
</script>

<div class="flex flex-col gap-5">
	<div role="tablist" aria-label={label} class="flex gap-1 border-b border-[var(--color-border)]">
		{#each tabs as tab, index (tab.id)}
			<button
				type="button"
				role="tab"
				id={`tab-${tab.id}`}
				aria-selected={active === tab.id}
				aria-controls={`panel-${tab.id}`}
				tabindex={active === tab.id ? 0 : -1}
				bind:this={buttons[index]}
				class="-mb-px min-h-11 border-b-2 border-transparent px-3 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)] aria-selected:border-[var(--color-accent)] aria-selected:font-medium aria-selected:text-[var(--color-text)]"
				onclick={() => (selected = tab.id)}
				onkeydown={(event) => move(event, index)}
			>
				{tab.label}
			</button>
		{/each}
	</div>

	<div
		role="tabpanel"
		id={`panel-${active}`}
		aria-labelledby={`tab-${active}`}
		tabindex="0"
		class="focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
	>
		{@render panel(active)}
	</div>
</div>
