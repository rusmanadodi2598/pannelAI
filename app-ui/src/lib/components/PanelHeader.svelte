<script lang="ts">
	// Panel header: the shell's controls, above the content column.
	//
	// Two decisions worth stating, because both are departures from the legacy panel:
	//
	//   The only control that opens the sidebar on a small screen is the trigger, and it is present at
	//   every breakpoint (the primitive layer already hides the rail on mobile).
	//
	//   The API base control copies a value an operator pastes into a client. It is a button with a live
	//   region announcing the copy, not a link, so there is nothing to navigate to and nothing to break.
	import { session } from '$lib/stores/session.svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { Moon, Sun } from '@lucide/svelte';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';

	let copied = $state(false);
	let copyFailed = $state(false);

	const apiBase = $derived(
		typeof location === 'undefined' ? '/api/v1' : `${location.origin}/api/v1`
	);

	async function copyApiBase(): Promise<void> {
		copyFailed = false;
		try {
			await navigator.clipboard.writeText(apiBase);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be denied (an insecure origin, or a browser policy). Saying so is
			// better than a control that appears to work.
			copyFailed = true;
			setTimeout(() => (copyFailed = false), 4000);
		}
	}
</script>

<header
	class="sticky top-0 z-20 flex items-center gap-2 border-b border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2"
	style="padding-left: max(0.75rem, env(safe-area-inset-left)); padding-right: max(0.75rem, env(safe-area-inset-right));"
>
	<!-- The trigger toggles the sidebar through the primitive layer's context. `onnavigaterequest` is
	     not forwarded here: on a phone the sidebar's own Sheet gives Escape, the overlay, and focus
	     containment, and a second close path would compete with it. -->
	<!-- The primitive layer's trigger is a 32px icon button. R-03 and SPEC-UI §8.7.2 require a 44px
	     touch target, and this control is the only way to open navigation on a phone. -->
	<Sidebar.Trigger class="size-11 shrink-0 lg:hidden" />

	<div class="flex min-w-0 items-center gap-2 text-xs">
		<span class="hidden font-medium text-[var(--color-text)] sm:inline">API base</span>
		<code
			class="min-w-0 truncate rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-1 text-[var(--color-text-muted)]"
			>{apiBase}</code
		>
		<button
			type="button"
			class="min-h-11 shrink-0 rounded-[var(--radius-sm)] px-2 text-[var(--color-accent)] underline underline-offset-2 hover:bg-[var(--color-surface-2)]"
			onclick={copyApiBase}
		>
			{copied ? 'Copied' : 'Copy'}
		</button>
		<span class="sr-only" role="status" aria-live="polite">
			{copied ? 'API base URL copied to the clipboard.' : ''}
			{copyFailed
				? 'The browser refused clipboard access. Select the URL and copy it manually.'
				: ''}
		</span>
	</div>

	<div class="ms-auto flex items-center gap-1.5">
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
			onclick={() => theme.toggle()}
			aria-label={theme.name === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
		>
			{#if theme.name === 'dark'}
				<Sun class="size-4" aria-hidden="true" />
			{:else}
				<Moon class="size-4" aria-hidden="true" />
			{/if}
			<span class="hidden sm:inline">{theme.name === 'dark' ? 'Light' : 'Dark'}</span>
		</button>

		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
			onclick={() => session.signOut()}
		>
			Sign out
		</button>
	</div>
</header>
