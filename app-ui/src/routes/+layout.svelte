<script lang="ts">
	// Root layout: theme and session bootstrap, then the shell.
	//
	// The gate lives here rather than in each screen, so a new screen cannot ship without the session
	// check. Unauthenticated requests see the login route only, and they carry the route they wanted with
	// them so signing in returns them to it (SPEC-UI §8.1).
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import AppShell from '$lib/components/AppShell.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { shouldCancelNavigation } from '$lib/dirty-guard';
	import { session } from '$lib/stores/session.svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { loginRedirectTarget, loginUrl } from '$lib/utils/redirect';
	import { onMount, type Snippet } from 'svelte';

	// The token layer. Imported here because this is the one component every route renders through, so
	// there is no screen that can load without it. Before this line existed the file was written and
	// tested but never loaded, and the whole panel rendered unstyled.
	import '../app.css';

	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname);
	const onLoginRoute = $derived(path === '/login');

	// What to come back to after signing in. The search string is part of it, so a filtered view survives
	// a session that expired mid-task.
	const requestedRoute = $derived(page.url.pathname + page.url.search);

	// §8.4.4 lives here for the same reason the session gate does: it is one rule over every screen, and
	// a screen that had to remember it could forget it. The forms register themselves with the guard, so
	// this is the only place that asks, and the asking happens whether the navigation came from a link,
	// `goto`, or the browser's own reload.
	beforeNavigate((navigation) => {
		if (shouldCancelNavigation(navigation, (message) => window.confirm(message))) {
			navigation.cancel();
		}
	});

	onMount(async () => {
		theme.init();
		await session.refresh();
	});

	// Redirect only after the status is known, so a slow status call cannot bounce a signed-in
	// operator to the login screen.
	$effect(() => {
		if (session.loading) return;

		if (session.requireLogin && !session.authenticated && !onLoginRoute) {
			// The destination travels in the query string, so there is no route id for `resolve()` to take.
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- validated query value, not a route id
			void goto(loginUrl(requestedRoute));
			return;
		}

		if (session.authenticated && onLoginRoute) {
			// The requested route is attacker-influenced, so it is validated rather than followed.
			const destination = loginRedirectTarget(
				page.url.searchParams.get('redirectTo'),
				resolve('/endpoint-keys')
			);
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- validated query value, not a route id
			void goto(destination);
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
