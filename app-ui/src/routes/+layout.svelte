<script lang="ts">
	// Root layout: theme and session bootstrap, then the shell.
	//
	// The gate lives here rather than in each screen, so a new screen cannot ship without the session
	// check. Unauthenticated requests see the login route only.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AppShell from '$lib/components/AppShell.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { session } from '$lib/stores/session.svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { onMount, type Snippet } from 'svelte';

	// The token layer. Imported here because this is the one component every route renders through, so
	// there is no screen that can load without it. Before this line existed the file was written and
	// tested but never loaded, and the whole panel rendered unstyled.
	import '../app.css';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);
	const onLoginRoute = $derived(path === '/login');

	onMount(async () => {
		theme.init();
		await session.refresh();
	});

	// Redirect only after the status is known, so a slow status call cannot bounce a signed-in
	// operator to the login screen.
	$effect(() => {
		if (session.loading) return;

		if (session.requireLogin && !session.authenticated && !onLoginRoute) {
			void goto(resolve('/login'));
			return;
		}

		if (session.authenticated && onLoginRoute) {
			void goto(resolve('/endpoint-keys'));
		}
	});
</script>

{#if session.loading}
	<div class="p-6">
		<StateMessage kind="loading" title="Checking your session" />
	</div>
{:else if session.requireLogin && !session.authenticated}
	{@render children()}
{:else}
	<AppShell>{@render children()}</AppShell>
{/if}
