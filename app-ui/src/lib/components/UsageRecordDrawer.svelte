<script lang="ts">
	// One request's usage record, with its captured log when capture is on (docs/SPEC-UI/001-SPEC-UI.md §6.5,
	// tab 2 detail).
	//
	// The detail has three capture states rather than two, because the API distinguishes them and collapsing
	// them would lose the difference: capture is off, capture is on but no log row exists (retention rotated
	// it out), and capture is on with a log. §6.5 asks for the first to be stated rather than shown as an
	// empty body area, and the second would otherwise look exactly like it.
	import CapturedBodies from '$lib/components/CapturedBodies.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getUsageRecord } from '$lib/api/usage';
	import { REQUEST_STATUS_LABELS } from '$lib/schemas/primitives';
	import type { UsageRecordDetail } from '$lib/schemas/usage';
	import { formatCount } from '$lib/schemas/usage-view';
	import { formatTimestamp } from '$lib/utils/time';

	let { requestId, onclose }: { requestId: string | null; onclose: () => void } = $props();

	let detail = $state<UsageRecordDetail | null>(null);
	let loading = $state(false);
	let error = $state<string | null>(null);

	// Reloads whenever the drawer is handed a different request, so the facts and the bodies always belong
	// to the row that was opened.
	$effect(() => {
		const id = requestId;
		if (!id) {
			detail = null;
			return;
		}
		void load(id);
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const result = await getUsageRecord(id);
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		detail = result.data;
	}
</script>

<Modal title={requestId ? `Request ${requestId}` : 'Request'} open={requestId !== null} {onclose}>
	{#if loading}
		<StateMessage kind="loading" title="Loading the request" />
	{:else if error}
		<StateMessage kind="error" title="The request could not be loaded" description={error} />
	{:else if detail}
		<div class="flex flex-col gap-5 text-sm">
			<dl class="grid grid-cols-2 gap-3">
				<div>
					<dt class="text-[var(--color-text-muted)]">Recorded at</dt>
					<dd>{formatTimestamp(detail.usage.ts)}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Status</dt>
					<dd>{REQUEST_STATUS_LABELS[detail.usage.status] ?? detail.usage.status}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Model</dt>
					<dd>{detail.usage.model}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Provider</dt>
					<dd>{detail.usage.provider_id}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Endpoint</dt>
					<dd>{detail.usage.endpoint_id ?? 'Not recorded'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Gateway key</dt>
					<dd>{detail.usage.gateway_key_id ?? 'Not recorded'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Combo</dt>
					<dd>{detail.usage.combo ?? 'Not routed through a combo'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Latency</dt>
					<dd>{formatCount(detail.usage.latency_ms)} ms</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Tokens in</dt>
					<dd>{formatCount(detail.usage.tokens_in)}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Tokens out</dt>
					<dd>{formatCount(detail.usage.tokens_out)}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Cache read / write</dt>
					<dd>
						{formatCount(detail.usage.tokens_cache_read)} /
						{formatCount(detail.usage.tokens_cache_write)}
					</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Cost (USD, estimated)</dt>
					<dd>{detail.usage.cost_usd}</dd>
				</div>
			</dl>

			{#if detail.usage.error_code}
				<p>
					Error code: <span class="font-medium">{detail.usage.error_code}</span>
				</p>
			{/if}

			<section class="flex flex-col gap-3">
				<h3 class="text-sm font-medium">Captured bodies</h3>

				{#if !detail.capture_enabled}
					<p class="text-sm text-[var(--color-text-muted)]">
						Capture is off, so the gateway recorded this request's outcome and not its bodies. The
						figures above come from the usage record, which is written either way.
					</p>
				{:else if !detail.log}
					<p class="text-sm text-[var(--color-text-muted)]">
						Capture is on, but the gateway has no stored log row for this request. It was either
						recorded before capture was turned on or has been rotated out by the retention window.
					</p>
				{:else}
					<CapturedBodies
						requestBody={detail.log.request_body}
						responseBody={detail.log.response_body}
						maxBytes={detail.log.capture_body_max_bytes}
					/>
				{/if}
			</section>
		</div>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={onclose}>Close</button
		>
	{/snippet}
</Modal>
