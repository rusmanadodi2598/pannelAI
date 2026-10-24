<script lang="ts">
	// The keys table inside the endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Split out of the drawer because the row carries eight fields and three actions, which is enough on
	// its own to push one file past the project line limit. The delete control is disabled when the API
	// would refuse the call, and the note beside it says why, so the operator is not left guessing at a
	// greyed-out button (R-26).
	//
	// The three actions are icon-only: each glyph comes from the one icon
	// map and the button carries the action's name as its accessible name and its title.
	//
	// Chosen and served are two separate columns on purpose. A key can be picked by rotation and then
	// lose the race to a failover, so "last chosen" and "last served" come apart, and the gap between
	// them is what the fix was for: it tells the operator a key is being selected but not carrying load.
	//
	// `rate_limited_until` is the one field here whose value is a duration rather than a fact: §6.2 asks for
	// a countdown, and a countdown needs a clock. The drawer owns `now` and passes it in, so the table stays
	// a renderer and the ticking lives with the thing that is open. The chosen and served times are read
	// against that same clock, so a row does not show a wait ticking down beside a stale absolute timestamp.
	//
	// The wait's provenance reads inside that same cell rather than in a column of its own, per §8.7: under
	// 768px every cell becomes a labelled line of the row card, so a ninth column is a ninth line in every
	// card on a phone, while a wait and the failure that caused it are one fact that belongs side by side.
	import PagingControls from '$lib/components/PagingControls.svelte';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';
	import { ENDPOINT_STATUS_ACTIVE, type EndpointKey } from '$lib/schemas/endpoint';
	import { pageOf } from '$lib/schemas/local-page';
	import { isLastActiveApiKey, parkProvenance } from '$lib/utils/routing';
	import { rowChecked } from '$lib/utils/selection';
	import { countdownText, elapsedText, formatTimestamp } from '$lib/utils/time';

	let {
		keys,
		authType,
		testing,
		now,
		ontest,
		onsettled,
		onremove,
		selected = [],
		allSelected = false,
		onselect
	}: {
		keys: EndpointKey[];
		authType: string;
		testing: string | null;
		/** The instant the countdown is measured against, moved by the drawer's tick. */
		now: number;
		ontest: (key: EndpointKey) => void;
		onsettled: (key: EndpointKey, status: 'active' | 'disabled') => void;
		onremove: (key: EndpointKey) => void;
		/** Ids selected in this drawer. Present alongside `onselect` to render the checkbox. */
		selected?: string[];
		/**
		 * Selecting here addresses the keys of this one endpoint. `all` therefore means every key
		 * this connection holds, never every key across providers: the drawer has no server scope
		 * for that, and a control that promised one would be counting a set it cannot see.
		 */
		/** Select-All is on, which contains every key this connection holds. */
		allSelected?: boolean;
		onselect?: (id: string, checked: boolean) => void;
	} = $props();

	const selectable = $derived(onselect !== undefined);

	// A connection can carry 100+ keys (one per account). Ten a page keeps the drawer readable on a phone.
	let requestedPage = $state(1);
	const view = $derived(pageOf(keys, requestedPage));

	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;
</script>

<div
	class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
>
	<table class="mobile-key-value-table w-full border-collapse">
		<caption class="sr-only">Keys on this endpoint</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Label</th>
				<th scope="col" class="px-3 py-2 font-medium">Key</th>
				<th scope="col" class="px-3 py-2 font-medium">Priority</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Last chosen</th>
				<th scope="col" class="px-3 py-2 font-medium">Last served</th>
				<th scope="col" class="px-3 py-2 font-medium">Errors</th>
				<th scope="col" class="px-3 py-2 font-medium">Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each view.rows as key (key.id)}
				{@const blocked = isLastActiveApiKey(keys, key.id, authType)}
				{@const isActive = key.status === ENDPOINT_STATUS_ACTIVE}
				{@const provenance = parkProvenance(key)}
				<tr class="border-t border-[var(--color-border)]">
					<td data-label="Label" class="px-3 py-2">
						{key.label || 'Unlabelled'}
						{#if key.tags?.length}
							<span class="ml-1 inline-flex flex-wrap gap-1">
								{#each key.tags as tag (tag)}
									<span
										class="rounded-[var(--radius-sm)] border border-[var(--color-border)] px-1 text-xs"
										>{tag}</span
									>
								{/each}
							</span>
						{/if}
					</td>
					<td data-label="Key" class="px-3 py-2">{key.key_hint}</td>
					<td data-label="Priority" class="px-3 py-2 tabular-nums">{key.priority}</td>
					<td data-label="Status" class="px-3 py-2">
						{key.status}{key.available ? '' : ', unavailable'}
						{#if key.rate_limited_until}
							<br />
							<span class="text-[var(--color-text-muted)]">
								rate limited until {formatTimestamp(key.rate_limited_until)}
								({countdownText(key.rate_limited_until, now)})
								{#if provenance !== ''}
									<br />
									<span>{provenance}</span>
								{/if}
							</span>
						{/if}
					</td>
					<td data-label="Last chosen" class="px-3 py-2">
						{key.last_selected_at ? elapsedText(key.last_selected_at, now) : 'Never chosen'}
					</td>
					<td data-label="Last served" class="px-3 py-2">
						{key.last_used_at ? elapsedText(key.last_used_at, now) : 'Never served'}
					</td>
					<td data-label="Errors" class="px-3 py-2 tabular-nums">{key.consecutive_errors}</td>
					<td data-label="Actions" class="px-3 py-2">
						<div class="flex flex-wrap items-center gap-1">
							{#if selectable}
								<!-- Selection sits with the actions you can take on this credential rather than on
									 a labelled line of its own once the row becomes a card below 768px. -->
								<label class="flex min-h-11 min-w-11 items-center justify-center">
									<input
										type="checkbox"
										class="size-4"
										aria-label={`Select ${key.label || key.key_hint} for bulk actions`}
										checked={rowChecked({ ids: selected, all: allSelected }, key.id)}
										onchange={(event) => onselect?.(key.id, event.currentTarget.checked)}
									/>
								</label>
							{/if}
							<Button
								variant="ghost"
								size="icon"
								aria-label="Test"
								title="Test"
								disabled={testing !== null}
								aria-busy={testing === key.id}
								onclick={() => ontest(key)}><TestIcon class="size-4" aria-hidden="true" /></Button
							>
							<Button
								variant="ghost"
								size="icon"
								aria-label={isActive ? 'Disable' : 'Enable'}
								title={isActive ? 'Disable' : 'Enable'}
								onclick={() => onsettled(key, isActive ? 'disabled' : 'active')}
							>
								{#if isActive}
									<DisableIcon class="size-4" aria-hidden="true" />
								{:else}
									<EnableIcon class="size-4" aria-hidden="true" />
								{/if}
							</Button>
							<Button
								variant="destructive-ghost"
								size="icon"
								aria-label="Delete"
								title="Delete"
								disabled={blocked}
								onclick={() => onremove(key)}
								><DeleteIcon class="size-4" aria-hidden="true" /></Button
							>
						</div>
						{#if blocked}
							<span class="text-[var(--color-text-muted)]"
								>Last active key: deleting leaves nothing to route with.</span
							>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

{#if view.pages > 1}
	<PagingControls
		page={view.page}
		lastPage={view.pages}
		onpage={(next) => (requestedPage = next)}
	/>
{/if}
