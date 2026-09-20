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
	// look like one. The three actions carry an accessible name that includes the row, because
	// "Edit / Test / Delete" repeated down a table is ambiguous to a screen reader even though it is
	// clear to the eye.
	import {
		PROXY_PROTOCOL_LABELS,
		proxyTestStateLabel,
		type Proxy,
		type ProxyProtocol
	} from '$lib/schemas/proxy';
	import { formatTimestamp } from '$lib/utils/time';

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

	const actionClass = 'min-h-11 underline';
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
						<div class="flex flex-wrap gap-x-3">
							<button
								type="button"
								class={actionClass}
								aria-label={`Edit ${proxy.label}`}
								onclick={() => onedit(proxy)}>Edit</button
							>
							<button
								type="button"
								class={actionClass}
								disabled={testingId === proxy.id}
								aria-label={`Test ${proxy.label}`}
								onclick={() => ontest(proxy)}
							>
								{testingId === proxy.id ? 'Testing' : 'Test'}
							</button>
							<button
								type="button"
								class={actionClass}
								aria-label={`Delete ${proxy.label}`}
								onclick={() => ondelete(proxy)}>Delete</button
							>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
