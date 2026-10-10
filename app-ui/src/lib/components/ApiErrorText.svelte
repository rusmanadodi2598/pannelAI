<script lang="ts">
	import { ApiError } from '$lib/api/errors';
	import RequestIdDisplay from '$lib/components/RequestIdDisplay.svelte';

	type ErrorValue = string | ApiError | null | undefined;

	let {
		error,
		prefix = '',
		suffix = ''
	}: {
		error: ErrorValue;
		prefix?: string;
		suffix?: string;
	} = $props();

	const message = $derived(typeof error === 'string' ? error : error?.message);
	const apiError = $derived(error instanceof ApiError ? error : null);
</script>

{#if message}
	<span>{prefix}{message}{suffix}<RequestIdDisplay {apiError} /></span>
{/if}
