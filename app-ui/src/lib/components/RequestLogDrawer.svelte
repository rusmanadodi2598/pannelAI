<script lang="ts">
	// The request log detail (docs/SPEC-UI/001-SPEC-UI.md §6.11, tab 1).
	//
	// The detail has three capture states rather than two, because the API distinguishes them and collapsing
	// them would lose the difference: capture is off, capture is on but nothing was stored, and capture is
	// on with a body. §6.11 asks for the first to be stated rather than shown as an empty body area.
	//
	// A body that ends with the truncation marker says so where it ends (§6.11), and §7.5.1 says it is
	// shown exactly as it was recorded, so the marker and the raw bytes are both kept.
	import CapturedBodies from '$lib/components/CapturedBodies.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getRequestLog } from '$lib/api/log';
	import { REQUEST_STATUS_LABELS } from '$lib/schemas/primitives';
	import type { LogDetail } from '$lib/schemas/log';
	import { bodyTruncated } from '$lib/schemas/log-view';
	import { formatCount } from '$lib/schemas/usage-view';
	import { formatTimestamp } from '$lib/utils/time';

	let { requestId, onclose }: { requestId: string | null; onclose: () => void } = $props();

	let detail = $state<LogDetail | null>(null);
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
		const result = await getRequestLog(id);
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
					<dd>{formatTimestamp(detail.ts)}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Status</dt>
					<dd>{REQUEST_STATUS_LABELS[detail.status] ?? detail.status}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Gateway key</dt>
					<dd>{detail.gateway_key_id ?? 'Not recorded'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Endpoint</dt>
					<dd>{detail.endpoint_id ?? 'Not recorded'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Model</dt>
					<dd>{detail.model ?? 'Not recorded'}</dd>
				</div>
				<div>
					<dt class="text-[var(--color-text-muted)]">Latency</dt>
					<dd>{formatCount(detail.latency_ms)} ms</dd>
				</div>
			</dl>

			{#if detail.error}
				<p>
					Error: <span class="font-medium">{detail.error}</span>
				</p>
			{/if}

			<section class="flex flex-col gap-3">
				<h3 class="text-sm font-medium">Captured bodies</h3>

				{#if !detail.capture_enabled}
					<p class="text-sm text-[var(--color-text-muted)]">
						Capture is off, so the gateway recorded this request's outcome and not its bodies. They
						can be turned on in Settings.
					</p>
				{:else if !detail.request_body && !detail.response_body}
					<p class="text-sm text-[var(--color-text-muted)]">
						Capture is on, but the gateway stored no body for this request: either it had nothing to
						store or the row has been rotated out of retention.
					</p>
				{:else}
					<CapturedBodies
						requestBody={detail.request_body}
						responseBody={detail.response_body}
						maxBytes={detail.capture_body_max_bytes}
					/>

					{#if bodyTruncated(detail.request_body)}
						<p class="text-xs text-[var(--color-text-muted)]">
							The request body was cut at the capture limit.
						</p>
					{/if}

					{#if bodyTruncated(detail.response_body)}
						<p class="text-xs text-[var(--color-text-muted)]">
							The response body was cut at the capture limit.
						</p>
					{/if}
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
