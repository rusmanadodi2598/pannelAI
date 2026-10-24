<script lang="ts">
	// The two filters the provider registry accepts, and the refresh that re-reads both (SPEC-UI §6.3).
	//
	// The typed term stays the screen's, because clearing the filters resets it from there. The category
	// row is chips rather than a select so every option is visible at once, and `aria-pressed` is the
	// chosen state, which the button both paints and announces (DESIGN.md §3.5).
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { CONTROL_ICONS } from '$lib/icons';
	import { PROVIDER_CATEGORIES } from '$lib/schemas/provider';

	const CATEGORY_FILTERS = [
		{ value: '', label: 'All' },
		...PROVIDER_CATEGORIES.map((value) => ({ value, label: value }))
	];

	const SearchIcon = CONTROL_ICONS.search.icon;

	type Props = {
		category: string;
		draft: string;
		searchError: string | null;
		oncategory: (value: string) => void;
		onsearch: () => void;
		onrefresh: () => void | Promise<void>;
	};

	let {
		category,
		draft = $bindable(''),
		searchError,
		oncategory,
		onsearch,
		onrefresh
	}: Props = $props();
</script>

<!-- Row 1: the search, then the refresh. From sm the two share one line; on a phone each takes the full width. -->
<div class="flex flex-col gap-2 sm:flex-row sm:items-start">
	<div class="min-w-0 sm:max-w-md sm:flex-1">
		<form
			class="flex min-w-0 items-center gap-2"
			onsubmit={(event) => {
				event.preventDefault();
				onsearch();
			}}
		>
			<input
				type="search"
				bind:value={draft}
				placeholder="Search name or id"
				aria-label="Search providers by name or id"
				aria-describedby={searchError ? 'provider-search-error' : undefined}
				aria-invalid={searchError !== null}
				class="min-h-11 min-w-0 flex-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			/>
			<Button type="submit" variant="outline">
				<SearchIcon class="size-4" aria-hidden="true" />
				Search
			</Button>
		</form>
		{#if searchError}
			<p id="provider-search-error" role="alert" class="mt-1 text-sm text-[var(--color-danger)]">
				{searchError}
			</p>
		{/if}
	</div>
	<div
		class="flex min-w-0 [&>div]:w-full [&_button]:w-full sm:w-auto sm:[&>div]:w-auto sm:[&_button]:w-auto"
	>
		<RefreshControl {onrefresh} />
	</div>
</div>

<!-- Row 2: the category chips span the full width, three to a row on a phone and all six on one line from sm. -->
<div role="group" aria-label="Filter by category" class="grid grid-cols-3 gap-2 sm:grid-cols-6">
	{#each CATEGORY_FILTERS as option (option.value)}
		<Button
			variant="toggle"
			class="w-full min-w-0 px-2"
			aria-pressed={category === option.value}
			onclick={() => oncategory(option.value)}
		>
			{option.label}
		</Button>
	{/each}
</div>
