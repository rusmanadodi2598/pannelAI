<script lang="ts">
	// Panel error view (docs/SPEC-UI/001-SPEC-UI.md §5.1, §8.2).
	//
	// The only error the panel raises on its own is a route that does not exist, so 404 is the case with
	// real copy: it names the requested path and offers the way back. Every other status renders from the
	// same shape without echoing the server's message, because a raw message can leak detail and §8.2
	// asks for one panel voice per code. The requested path is rendered in its own element rather than
	// interpolated into a sentence so a long address wraps instead of overflowing the page (R-03).
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import StateMessage from '$lib/components/StateMessage.svelte';

	const pathname = $derived(page.url.pathname);
	const status = $derived(page.status);

	// One entry per status the panel can explain, with a fallback that still names the status. The
	// fallback is the reason this is a lookup rather than a chain of comparisons: an unlisted status has
	// a defined treatment instead of an empty screen.
	const COPY: Record<number, { title: string; description: string }> = {
		403: {
			title: 'This screen is not available to you',
			description: 'The gateway refused the request. Sign in again if the session may have expired.'
		},
		404: {
			title: 'No screen is routed here',
			description: 'The address may be mistyped, or the screen may not exist yet.'
		}
	};

	const copy = $derived(
		COPY[status] ?? {
			title: 'The panel could not load this screen',
			description:
				'The gateway did not answer successfully. Try again, and check the console log if it keeps failing.'
		}
	);
</script>

<section class="flex max-w-2xl flex-col gap-4">
	<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>

	<StateMessage kind="error" title={`Error ${status}`} description={copy.description}>
		{#snippet action()}
			<a
				href={resolve('/endpoint-keys')}
				class="inline-flex min-h-11 items-center rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)]"
			>
				Back to endpoint keys
			</a>
		{/snippet}
	</StateMessage>

	{#if status === 404}
		<p class="text-sm text-[var(--color-text-muted)]">
			Requested path
			<code
				class="break-all rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] px-1.5 py-0.5 font-mono text-xs text-[var(--color-text)]"
				>{pathname}</code
			>
		</p>
	{/if}
</section>
