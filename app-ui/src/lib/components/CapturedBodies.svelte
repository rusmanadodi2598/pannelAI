<script lang="ts">
	// The captured request and response bodies of one request (docs/SPEC-UI/001-SPEC-UI.md §6.5 detail,
	// §7.5.1).
	//
	// Rendered verbatim, in text nodes. §7.5.1 is explicit that a stored payload is evidence: the panel does
	// not reformat, re-indent, or redact it, and it never reaches for `{@html}`, so the browser escapes it.
	// The byte cap is printed because a body that stops mid-token has to say why it stops, rather than
	// looking like a payload that was always that short.
	//
	// Shared with the `/logs` screen, which renders the same shape for the same reason.
	import { formatCount } from '$lib/schemas/usage-view';

	type Props = {
		requestBody?: string;
		responseBody?: string;
		maxBytes: number;
	};

	let { requestBody, responseBody, maxBytes }: Props = $props();

	const hasBodies = $derived(
		(requestBody !== undefined && requestBody !== '') ||
			(responseBody !== undefined && responseBody !== '')
	);
</script>

<div class="flex flex-col gap-4">
	{#if !hasBodies}
		<p class="text-sm text-[var(--color-text-muted)]">
			No bodies were stored for this request. Capture was on when it was recorded, so the gateway
			either had nothing to store or the row has been rotated out of retention.
		</p>
	{:else}
		{#if requestBody}
			<div class="flex flex-col gap-1">
				<h3 class="text-sm font-medium">Request body</h3>
				<pre
					class="max-h-64 overflow-auto rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] p-3 text-xs">{requestBody}</pre>
			</div>
		{/if}

		{#if responseBody}
			<div class="flex flex-col gap-1">
				<h3 class="text-sm font-medium">Response body</h3>
				<pre
					class="max-h-64 overflow-auto rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] p-3 text-xs">{responseBody}</pre>
			</div>
		{/if}
	{/if}

	<p class="text-xs text-[var(--color-text-muted)]">
		Each body is captured up to {formatCount(maxBytes)} bytes and is shown exactly as it was recorded.
	</p>
</div>
