<script lang="ts">
	import CopyButton from '$lib/components/CopyButton.svelte';
	import type { ApiError } from '$lib/api/errors';

	let {
		requestId,
		apiError
	}: {
		requestId?: string | null;
		apiError?: Pick<ApiError, 'code' | 'requestId'> | null;
	} = $props();
	const displayedRequestId = $derived(
		requestId ?? (apiError?.code === 'INTERNAL_ERROR' ? apiError.requestId : undefined)
	);
</script>

{#if displayedRequestId}
	<span class="inline-flex flex-wrap items-center gap-2 text-sm">
		<span class="text-[var(--color-text-muted)]">Request ID</span>
		<code class="break-all">{displayedRequestId}</code>
		<CopyButton value={displayedRequestId} label="Copy request ID" touchTarget />
	</span>
{/if}
