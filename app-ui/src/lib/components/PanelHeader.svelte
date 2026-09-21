<script lang="ts">
	// Panel header: the shell's controls, above the content column.
	//
	// Two decisions worth stating, because both are departures from the legacy panel:
	//
	//   The only control that opens the sidebar on a small screen is the trigger, and it is present at
	//   every breakpoint (the primitive layer already hides the rail on mobile).
	//
	//   The API base is one button that opens ApiBaseDialog, rather than an address and a copy control
	//   squeezed into the strip. The three forms a client is configured with are in the dialog, each with
	//   the copy control beside the value it copies (SPEC-UI §5.2).
	import ApiBaseDialog from '$lib/components/ApiBaseDialog.svelte';
	import { session } from '$lib/stores/session.svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { Moon, Sun } from '@lucide/svelte';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';

	let baseOpen = $state(false);
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

	<button
		type="button"
		class="min-h-11 shrink-0 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
		onclick={() => (baseOpen = true)}
		aria-haspopup="dialog"
	>
		API base
	</button>

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

<!-- A sibling of the header rather than a child of it, so the strip's own layout is the same whether the
     dialog is open, closed, or rendered without a native modal implementation. -->
<ApiBaseDialog open={baseOpen} onclose={() => (baseOpen = false)} />
