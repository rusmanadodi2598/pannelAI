<script lang="ts">
	// The keys table inside the endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Split out of the drawer because the row carries seven fields and three actions, which is enough on
	// its own to push one file past the project line limit. The delete control is disabled when the API
	// would refuse the call, and the note beside it says why, so the operator is not left guessing at a
	// greyed-out button (R-26).
	//
	// The three actions are icon-only (owner directive, 2026-09-24): each glyph comes from the one icon
	// map and the button carries the action's name as its accessible name and its title.
	//
	// `rate_limited_until` is the one field here whose value is a duration rather than a fact: §6.2 asks for
	// a countdown, and a countdown needs a clock. The drawer owns `now` and passes it in, so the table stays
	// a renderer and the ticking lives with the thing that is open.
	import { ENDPOINT_STATUS_ACTIVE, type EndpointKey } from '$lib/schemas/endpoint';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { isLastActiveApiKey } from '$lib/utils/routing';
	import { countdownText, formatTimestamp } from '$lib/utils/time';

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

	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	// One target size and one hover wash for every action, so the cell reads as a set. Each variant
	// carries exactly one text colour: two competing `text-*` utilities resolve by stylesheet order, and
	// the muted one silently won the destructive button's colour (measured live 2026-09-24).
	const actionBase =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-2)] disabled:opacity-50';
	const actionClass = `${actionBase} text-[var(--color-text-muted)] hover:text-[var(--color-text)]`;
	const dangerClass = `${actionBase} text-[var(--color-danger)] hover:text-[var(--color-danger)]`;
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[46rem] border-collapse">
		<caption class="sr-only">Keys on this endpoint</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Label</th>
				<th scope="col" class="px-3 py-2 font-medium">Key</th>
				<th scope="col" class="px-3 py-2 font-medium">Priority</th>
				<th scope="col" class="px-3 py-2 font-medium">Status</th>
				<th scope="col" class="px-3 py-2 font-medium">Last used</th>
				<th scope="col" class="px-3 py-2 font-medium">Errors</th>
				<th scope="col" class="px-3 py-2 font-medium">Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each keys as key (key.id)}
				{@const blocked = isLastActiveApiKey(keys, key.id, authType)}
				{@const isActive = key.status === ENDPOINT_STATUS_ACTIVE}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">{key.label || 'Unlabelled'}</td>
					<td class="px-3 py-2">{key.key_hint}</td>
					<td class="px-3 py-2 tabular-nums">{key.priority}</td>
					<td class="px-3 py-2">
						{key.status}{key.available ? '' : ', unavailable'}
						{#if key.rate_limited_until}
							<br />
							<span class="text-[var(--color-text-muted)]">
								rate limited until {formatTimestamp(key.rate_limited_until)}
								({countdownText(key.rate_limited_until, now)})
							</span>
						{/if}
					</td>
					<td class="px-3 py-2">{key.last_used_at ?? 'Never'}</td>
					<td class="px-3 py-2 tabular-nums">{key.consecutive_errors}</td>
					<td class="px-3 py-2">
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
								>Last active key, so deleting it would leave the endpoint with nothing to route
								with.</span
							>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
