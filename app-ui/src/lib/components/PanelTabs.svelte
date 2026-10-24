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
	import { untrack, type Snippet } from 'svelte';
	import { Button } from '$lib/primitives/button/index.js';

	type Tab = { id: string; label: string };

	let {
		tabs,
		label,
		panel,
		initial,
		onchange
	}: {
		tabs: Tab[];
		label: string;
		panel: Snippet<[string]>;
		/** The tab a deep link asked for, until the operator chooses one. */
		initial?: string;
		/**
		 * Told which tab is now showing, for a screen whose submit path differs by tab.
		 *
		 * The selection lives here rather than in the caller, so a caller that needs it for anything
		 * beyond rendering the panel has to hear about it: a dialog whose two tabs hold two different
		 * fields cannot read the choice back out of the markup.
		 */
		onchange?: (id: string) => void;
	} = $props();

	// `selected` starts unset and falls back to the deep-linked tab, then the first tab, so the active tab
	// follows the tab list if it is rendered with a different one instead of freezing the first render.
	let selected = $state<string | null>(null);
	const active = $derived(
		selected ?? (tabs.some((tab) => tab.id === initial) ? initial : tabs[0]?.id) ?? ''
	);

	// A deep link can change while the screen is mounted: a row action on one tab opens another. When the
	// caller's `initial` changes, the selection follows it. Clearing `selected` returns the choice to the
	// deep link instead of freezing the tab the operator was on.
	let lastInitial = untrack(() => initial);
	$effect(() => {
		if (initial === lastInitial) return;
		lastInitial = initial;
		selected = null;
	});
	// `bind:ref` refuses an unset slot against the primitive's own `null` fallback, so the array is sized to
	// the tabs before any tab binds into it. The tab list is static per screen, which is why one pass fits.
	// svelte-ignore state_referenced_locally
	let buttons = $state<(HTMLButtonElement | null)[]>(tabs.map(() => null));

	// Every path that moves the selection goes through here, so a caller listening to `onchange` cannot
	// miss the keyboard moves that do not pass through a click.
	function select(id: string): void {
		selected = id;
		onchange?.(id);
	}

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
		select(tabs[next].id);
		buttons[next]?.focus();
	}
</script>

<div class="flex flex-col gap-5">
	<div
		role="tablist"
		aria-label={label}
		class="flex gap-1 overflow-x-auto border-b border-[var(--color-border)]"
	>
		{#each tabs as tab, index (tab.id)}
			<Button
				variant="ghost"
				role="tab"
				id={`tab-${tab.id}`}
				aria-selected={active === tab.id}
				aria-controls={`panel-${tab.id}`}
				tabindex={active === tab.id ? 0 : -1}
				bind:ref={buttons[index]}
				class="-mb-px shrink-0 border-b-2 px-3 font-normal aria-selected:font-medium aria-selected:border-[var(--color-accent)] aria-selected:text-[var(--color-text)]"
				onclick={() => select(tab.id)}
				onkeydown={(event) => move(event, index)}
			>
				{tab.label}
			</Button>
		{/each}
	</div>

	<div role="tabpanel" id={`panel-${active}`} aria-labelledby={`tab-${active}`} tabindex="0">
		{@render panel(active)}
	</div>
</div>
