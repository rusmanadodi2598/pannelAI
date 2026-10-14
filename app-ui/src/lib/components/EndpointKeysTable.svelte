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
	import { ENDPOINT_STATUS_ACTIVE, type EndpointKey } from '$lib/schemas/endpoint';
	import { pageOf } from '$lib/schemas/local-page';
	import { CONTROL_ICONS } from '$lib/icons';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { isLastActiveApiKey } from '$lib/utils/routing';
	import { countdownText, elapsedText, formatTimestamp } from '$lib/utils/time';

	let {
		keys,
		authType,
		testing,
		now,
		ontest,
		onsettled,
		onremove
	}: {
		keys: EndpointKey[];
		authType: string;
		testing: string | null;
		/** The instant the countdown is measured against, moved by the drawer's tick. */
		now: number;
		ontest: (key: EndpointKey) => void;
		onsettled: (key: EndpointKey, status: 'active' | 'disabled') => void;
		onremove: (key: EndpointKey) => void;
	} = $props();

	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

	// A connection can carry 100+ keys (one per account). Ten a page keeps the drawer readable on a phone.
	let requestedPage = $state(1);
	const view = $derived(pageOf(keys, requestedPage));

	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	// One target size and one hover wash for every action, so the cell reads as a set. Each variant
	// carries exactly one text colour: two competing `text-*` utilities resolve by stylesheet order, and
	// the muted one silently wins the destructive button's colour.
	const actionBase =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-2)] disabled:opacity-50';
	const actionClass = `${actionBase} text-[var(--color-text-muted)] hover:text-[var(--color-text)]`;
	const dangerClass = `${actionBase} text-[var(--color-danger)] hover:text-[var(--color-danger)]`;
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
				<tr class="border-t border-[var(--color-border)]">
					<td data-label="Label" class="px-3 py-2">{key.label || 'Unlabelled'}</td>
					<td data-label="Key" class="px-3 py-2">{key.key_hint}</td>
					<td data-label="Priority" class="px-3 py-2 tabular-nums">{key.priority}</td>
					<td data-label="Status" class="px-3 py-2">
						{key.status}{key.available ? '' : ', unavailable'}
						{#if key.rate_limited_until}
							<br />
							<span class="text-[var(--color-text-muted)]">
								rate limited until {formatTimestamp(key.rate_limited_until)}
								({countdownText(key.rate_limited_until, now)})
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
							<button
								type="button"
								class={actionClass}
								aria-label="Test"
								title="Test"
								disabled={testing !== null}
								onclick={() => ontest(key)}><TestIcon class="size-4" aria-hidden="true" /></button
							>
							<button
								type="button"
								class={actionClass}
								aria-label={isActive ? 'Disable' : 'Enable'}
								title={isActive ? 'Disable' : 'Enable'}
								onclick={() => onsettled(key, isActive ? 'disabled' : 'active')}
							>
								{#if isActive}
									<DisableIcon class="size-4" aria-hidden="true" />
								{:else}
									<EnableIcon class="size-4" aria-hidden="true" />
								{/if}
							</button>
							<button
								type="button"
								class={dangerClass}
								aria-label="Delete"
								title="Delete"
								disabled={blocked}
								onclick={() => onremove(key)}
								><DeleteIcon class="size-4" aria-hidden="true" /></button
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
	<div class="flex flex-wrap items-center gap-3 pt-2 text-sm">
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-1 px-3 underline disabled:opacity-50"
			disabled={view.page <= 1}
			onclick={() => (requestedPage = view.page - 1)}
		>
			<PreviousIcon class="size-4" aria-hidden="true" />
			Previous
		</button>
		<span class="text-[var(--color-text-muted)]">Page {view.page} of {view.pages}</span>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-1 px-3 underline disabled:opacity-50"
			disabled={view.page >= view.pages}
			onclick={() => (requestedPage = view.page + 1)}
		>
			Next
			<NextIcon class="size-4" aria-hidden="true" />
		</button>
	</div>
{/if}
