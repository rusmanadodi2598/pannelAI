<script lang="ts">
	// The quota cards (docs/SPEC-UI/001-SPEC-UI.md §6.6; card-per-provider reshape 2026-09-26,
	// docs/PORT/005-PORT-QUOTA-CARDS.md; server-driven paging docs/PORT/006-PORT-QUOTA-PAGING.md):
	// one card per provider in first-seen order, its connections listed inside a body that scrolls, a
	// header that folds, a checkbox that feeds the bulk fold bar, and a pager that walks pages the gateway
	// performs: the windows prop is already the page's groups, five to a page. The cards are grouped from the
	// union of the page's accounts (windows and `published`), so a provider that has an account but no
	// counted window yet still gets a card: the account the operator came to read is not hidden behind a
	// window that has not filled (the windowless-account gap).
	//
	// The provider's own answer rides in on the same read and is what the card body puts first; the gateway
	// counted windows are reduced to one summary line per connection there. The percentage a row prints
	// is percent USED and the bar's colour is the REMAINING share (quota-geometry.ts, in the panel's own
	// tokens). Both conventions are the screen's, not reinvented here.
	//
	// A window the gateway recorded without a provider (the credential-free lane's virtual endpoint) cannot
	// sit under a provider heading. It gets its own card, whose body opens by saying the counts are local.
	// The fold and selection sets are plain string arrays: the reactive-set lint rule this file once tripped
	// (pass 004) is avoided by never holding a Set or Map in component state at all.
	import { CONTROL_ICONS } from '$lib/icons';
	import {
		QUOTA_SOURCE_EXPLANATIONS,
		quotaCardGroups,
		type QuotaCardGroup,
		type QuotaWindow
	} from '$lib/schemas/quota';
	import {
		publishedEntriesByEndpoint,
		type PublishedQuotaUsage
	} from '$lib/schemas/quota-published';
	import QuotaCardBody from './QuotaCardBody.svelte';

	let {
		windows,
		labels,
		published,
		publishedNote,
		provider,
		now,
		page,
		pageCount,
		onpagechange
	}: {
		windows: QuotaWindow[];
		labels: Map<string, string>;
		/** The provider's answers the collection read carried for this page's accounts. */
		published: PublishedQuotaUsage[];
		/** The gateway's sentence for a provider cache it could not read at all, or null when there is none. */
		publishedNote: string | null;
		/** The provider the toolbar narrowed the page to, or empty for all of them. */
		provider: string;
		now: number;
		page: number;
		pageCount: number;
		onpagechange: (next: number) => void;
	} = $props();

	// Grouped from the union of the page's accounts, not from its windows: a provider with an account but no
	// counted window still gets a card (quota.ts, the windowless-account gap). First-seen order comes off the
	// wire there, so two identical reads put every card in the same place.
	const groups = $derived(quotaCardGroups(windows, published));

	// The provider filter narrows the cards the page already holds. The read stays server-paged and
	// unfiltered (006 D1): a filter that also moved the wire page would make the pager answer to it.
	const visibleGroups = $derived(
		provider === '' ? groups : groups.filter((group) => group.provider === provider)
	);

	// Keyed by endpoint, so a connection shows its own provider's number and never its neighbour's.
	const publishedByEndpoint = $derived(publishedEntriesByEndpoint(published));

	let folded: string[] = $state([]);
	let selected: string[] = $state([]);

	// A bulk action must never reach across pages unseen (005 D4): the checkboxes the operator cannot
	// see right now are not theirs to act on, so the selection starts clean whenever the pager lands
	// on another server page. A poll over the SAME page must not wipe it, which is why the effect
	// tracks the page and nothing else.
	$effect(() => {
		void page;
		selected = [];
	});

	function isFolded(key: string): boolean {
		return folded.includes(key);
	}

	function isSelected(key: string): boolean {
		return selected.includes(key);
	}

	function toggleFold(key: string): void {
		folded = isFolded(key) ? folded.filter((entry) => entry !== key) : [...folded, key];
	}

	function toggleSelect(key: string, on: boolean): void {
		if (on) {
			selected = isSelected(key) ? selected : [...selected, key];
		} else {
			selected = selected.filter((entry) => entry !== key);
		}
	}

	function foldSelected(): void {
		folded = [...folded, ...selected.filter((entry) => !folded.includes(entry))];
	}

	function unfoldSelected(): void {
		folded = folded.filter((entry) => !selected.includes(entry));
	}

	function cardName(group: QuotaCardGroup): string {
		return group.provider || 'No provider';
	}

	function cardKey(group: QuotaCardGroup): string {
		return group.provider || 'no-provider';
	}

	// A folded card's header is its whole story, so the counts name both dimensions the body would show:
	// the accounts on the card and the windows this gateway counted across them.
	function countsText(group: QuotaCardGroup): string {
		const endpoints = group.endpoints.length;
		const total = group.endpoints.reduce((sum, endpoint) => sum + endpoint.windows.length, 0);
		return `${endpoints} endpoint${endpoints === 1 ? '' : 's'}, ${total} window${total === 1 ? '' : 's'}`;
	}

	const FoldIcon = CONTROL_ICONS.fold.icon;
	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;
</script>

<div class="flex flex-col gap-4">
	{#if publishedNote !== null && publishedNote !== ''}
		<!-- Said once, above the cards: the counted windows underneath are still the gateway's own. -->
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{publishedNote}</p>
	{/if}

	{#each visibleGroups as group (cardKey(group))}
		{@const key = cardKey(group)}
		{@const name = cardName(group)}
		{@const bodyId = `quota-card-body-${key}`}
		<section
			aria-label={name}
			class="flex flex-col overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)]"
		>
			<div class="flex items-center gap-1 p-2">
				<!-- The label is the tap target at 44 px; the box inside stays the native size the rest of
				     the panel's forms use. -->
				<label class="flex min-h-11 min-w-11 items-center justify-center">
					<input
						type="checkbox"
						class="size-4"
						aria-label={`Select ${name} for bulk folding`}
						checked={isSelected(key)}
						onchange={(event) => toggleSelect(key, event.currentTarget.checked)}
					/>
				</label>
				<div class="flex min-w-0 flex-1 flex-col justify-center">
					<h3 class="truncate text-sm font-medium">{name}</h3>
					<span class="text-xs tabular-nums text-[var(--color-text-muted)]"
						>{countsText(group)}</span
					>
				</div>
				<button
					type="button"
					class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-md)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)]"
					aria-label={`Fold ${name}`}
					aria-expanded={!isFolded(key)}
					aria-controls={bodyId}
					title={`Fold ${name}`}
					onclick={() => toggleFold(key)}
				>
					<FoldIcon class="size-4 {isFolded(key) ? '' : 'rotate-180'}" aria-hidden="true" />
				</button>
			</div>

			{#if !isFolded(key)}
				<div
					id={bodyId}
					class="flex max-h-80 flex-col gap-3 overflow-y-auto border-t border-[var(--color-border)] p-3"
				>
					<QuotaCardBody {group} {labels} published={publishedByEndpoint} {now} />
				</div>
			{/if}
		</section>
	{/each}

	{#if provider !== '' && visibleGroups.length === 0}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">
			No {provider} card on this page. Choose another provider, or turn the page.
		</p>
	{/if}

	{#if selected.length > 0}
		<div
			role="status"
			class="flex flex-wrap items-center justify-between gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-2"
		>
			<span class="text-sm">{selected.length} selected</span>
			<div class="flex gap-2">
				<button
					type="button"
					class="inline-flex min-h-11 items-center rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 text-sm"
					onclick={foldSelected}
				>
					Fold selected
				</button>
				<button
					type="button"
					class="inline-flex min-h-11 items-center rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 text-sm"
					onclick={unfoldSelected}
				>
					Unfold selected
				</button>
			</div>
		</div>
	{/if}

	{#if groups.length > 0}
		<nav class="flex items-center justify-between gap-2" aria-label="Quota card pages">
			<button
				type="button"
				class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-md)] border border-[var(--color-border)] text-[var(--color-text-muted)] disabled:opacity-50"
				aria-label="Previous page"
				disabled={page <= 1}
				onclick={() => onpagechange(page - 1)}
			>
				<PreviousIcon class="size-4" aria-hidden="true" />
			</button>
			<span class="text-sm tabular-nums">Page {page} of {pageCount}</span>
			<button
				type="button"
				class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-md)] border border-[var(--color-border)] text-[var(--color-text-muted)] disabled:opacity-50"
				aria-label="Next page"
				disabled={page >= pageCount}
				onclick={() => onpagechange(page + 1)}
			>
				<NextIcon class="size-4" aria-hidden="true" />
			</button>
		</nav>
	{/if}
	<div class="flex flex-col gap-1 text-sm text-[var(--color-text-muted)]">
		<p>Computed: {QUOTA_SOURCE_EXPLANATIONS.computed}</p>
		<p>Reported: {QUOTA_SOURCE_EXPLANATIONS.reported}</p>
	</div>
</div>
