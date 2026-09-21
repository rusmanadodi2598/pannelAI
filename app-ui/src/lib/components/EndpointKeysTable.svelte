<script lang="ts">
	// The keys table inside the endpoint detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// Split out of the drawer because the row carries seven fields and three actions, which is enough on
	// its own to push one file past the project line limit. The delete control is disabled when the API
	// would refuse the call, and the note beside it says why, so the operator is not left guessing at a
	// greyed-out button (R-26).
	//
	// `rate_limited_until` is the one field here whose value is a duration rather than a fact: §6.2 asks for
	// a countdown, and a countdown needs a clock. The drawer owns `now` and passes it in, so the table stays
	// a renderer and the ticking lives with the thing that is open.
	import { ENDPOINT_STATUS_ACTIVE, type EndpointKey } from '$lib/schemas/endpoint';
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
						<div class="flex flex-wrap gap-2">
							<button
								type="button"
								class="min-h-11 underline disabled:opacity-50"
								disabled={testing !== null}
								onclick={() => ontest(key)}>Test</button
							>
							<button
								type="button"
								class="min-h-11 underline"
								onclick={() => onsettled(key, isActive ? 'disabled' : 'active')}
								>{isActive ? 'Disable' : 'Enable'}</button
							>
							<button
								type="button"
								class="min-h-11 underline disabled:opacity-50"
								disabled={blocked}
								onclick={() => onremove(key)}>Delete</button
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
