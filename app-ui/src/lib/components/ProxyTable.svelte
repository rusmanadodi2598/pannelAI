<script lang="ts">
	// The proxy pool table (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// Columns follow the decision the operator makes here: which candidate is failing, and whether it
	// is in the pool at all. Label and address identify the row, the test column says what the last
	// probe found, and Enabled says whether the row is a live candidate.
	//
	// The password is never rendered. The wire carries `has_password` and nothing else, so the cell
	// says whether one is stored rather than showing a value the panel does not have.
	//
	// A row that was never tested reads "Not tested", which is not the same as a failure and must not
	// look like one. The three actions are icon-only (owner directive, 2026-09-26): the glyph comes
	// from the row-action map, and the accessible name carries the row, because "Edit / Test / Delete"
	// repeated down a table is ambiguous to a screen reader even with tooltips. While a probe is in
	// flight the row's Test button is disabled, which is how the other tables show the same wait.
	import {
		PROXY_PROTOCOL_LABELS,
		proxyTestStateLabel,
		type Proxy,
		type ProxyProtocol
	} from '$lib/schemas/proxy';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { formatTimestamp } from '$lib/utils/time';

	const EditIcon = ROW_ACTION_ICONS.edit.icon;
	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	let {
		proxies,
		testingId,
		onedit,
		ontest,
		ondelete
	}: {
		proxies: Proxy[];
		/** The row whose probe is in flight, so only that row shows the wait. */
		testingId: string | null;
		onedit: (proxy: Proxy) => void;
		ontest: (proxy: Proxy) => void;
		ondelete: (proxy: Proxy) => void;
	} = $props();

	function protocolLabel(protocol: string): string {
		return PROXY_PROTOCOL_LABELS[protocol as ProxyProtocol] ?? protocol;
	}

	// One target size and one hover wash for every action, so the cell reads as a set. Each variant
	// carries exactly one text colour: two competing `text-*` utilities resolve by stylesheet order,
	// and the muted one silently won the destructive button's colour (measured live 2026-09-24).
	const actionBase =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-2)] disabled:opacity-50';
	const actionClass = `${actionBase} text-[var(--color-text-muted)] hover:text-[var(--color-text)]`;
	const dangerClass = `${actionBase} text-[var(--color-danger)] hover:text-[var(--color-danger)]`;
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[62rem] border-collapse text-sm">
		<caption class="sr-only">
			Proxy pool candidates, with the last connectivity test for each
		</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Label</th>
				<th scope="col" class="px-3 py-2 font-medium">Protocol</th>
				<th scope="col" class="px-3 py-2 font-medium">Host</th>
				<th scope="col" class="px-3 py-2 font-medium">Port</th>
				<th scope="col" class="px-3 py-2 font-medium">Username</th>
				<th scope="col" class="px-3 py-2 font-medium">Password</th>
				<th scope="col" class="px-3 py-2 font-medium">Enabled</th>
				<th scope="col" class="px-3 py-2 font-medium">Last test</th>
				<th scope="col" class="px-3 py-2 font-medium">Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each proxies as proxy (proxy.id)}
				<tr class="border-t border-[var(--color-border)] align-top">
					<td class="px-3 py-2 font-medium">{proxy.label}</td>
					<td class="px-3 py-2">{protocolLabel(proxy.protocol)}</td>
					<td class="px-3 py-2">{proxy.host}</td>
					<td class="px-3 py-2 tabular-nums">{proxy.port}</td>
					<td class="px-3 py-2">{proxy.username || 'None'}</td>
					<td class="px-3 py-2">{proxy.has_password ? 'Set' : 'Not set'}</td>
					<td class="px-3 py-2">{proxy.enabled ? 'Enabled' : 'Disabled'}</td>
					<td class="px-3 py-2">
						{#if proxy.status}
							<span>{proxyTestStateLabel(proxy.status.state)} in {proxy.status.latency_ms}ms</span>
							{#if proxy.status.checked_at}
								<span class="block text-xs text-[var(--color-text-muted)]">
									{formatTimestamp(proxy.status.checked_at)}
								</span>
							{/if}
							{#if proxy.status.message}
								<span class="block text-xs text-[var(--color-text-muted)]">
									{proxy.status.message}
								</span>
							{/if}
						{:else}
							<span class="text-[var(--color-text-muted)]">Not tested</span>
						{/if}
					</td>
					<td class="px-3 py-2">
						<div class="flex flex-wrap gap-x-1">
							<button
								type="button"
								class={actionClass}
								aria-label={`Edit ${proxy.label}`}
								title={`Edit ${proxy.label}`}
								onclick={() => onedit(proxy)}
							>
								<EditIcon class="size-4" aria-hidden="true" />
							</button>
							<button
								type="button"
								class={actionClass}
								disabled={testingId === proxy.id}
								aria-label={`Test ${proxy.label}`}
								title={`Test ${proxy.label}`}
								onclick={() => ontest(proxy)}
							>
								<TestIcon class="size-4" aria-hidden="true" />
							</button>
							<button
								type="button"
								class={dangerClass}
								aria-label={`Delete ${proxy.label}`}
								title={`Delete ${proxy.label}`}
								onclick={() => ondelete(proxy)}
							>
								<DeleteIcon class="size-4" aria-hidden="true" />
							</button>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
