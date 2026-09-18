<script lang="ts">
	// Panel shell: sidebar, header, and the content frame.
	//
	// Navigation follows src/lib/navigation.ts, which is where the owner's screen list lives. An item whose
	// screen is not built yet renders as inert text with a Planned chip, because a nav item pointing at a
	// missing route is exactly what R-24 forbids and the data model makes it unrepresentable.
	//
	// Breakpoint behaviour comes from DESIGN.md §8. The primitive layer already owns the hard part: it
	// swaps to a Sheet drawer below 768px and to a collapsible rail above it. What this component adds is
	// the initial state that matches the viewport, so a tablet starts as a rail instead of a full sidebar.
	//
	// The shell is `h-dvh`, not `min-h-screen`: 100vh is taller than the visible area under mobile browser
	// chrome, which is what clips a sticky header on a phone.
	import PanelHeader from '$lib/components/PanelHeader.svelte';
	import PanelSidebar from '$lib/components/PanelSidebar.svelte';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';
	import { readSidebarCookie, resolveSidebarOpen } from '$lib/stores/sidebar';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	// A tablet gets the icon rail and a desktop gets the full sidebar, unless the operator has chosen.
	// The primitive layer owns the state and writes the preference cookie itself, so this is the one
	// thing the application adds: the initial value that matches the viewport on a first visit.
	let open = $state(
		typeof document === 'undefined'
			? true
			: resolveSidebarOpen(readSidebarCookie(document.cookie), window.innerWidth >= 1024)
	);
</script>

<Sidebar.Provider bind:open>
	<div class="flex h-dvh w-full overflow-hidden bg-[var(--color-surface)]">
		<PanelSidebar />

		<div class="flex min-w-0 flex-1 flex-col">
			<PanelHeader />
			<!-- Only this column scrolls, so the header stays put and a long table does not drag the
			     whole shell with it. -->
			<main
				class="min-w-0 flex-1 overflow-y-auto p-4 lg:p-6"
				style="padding-bottom: max(1rem, env(safe-area-inset-bottom));"
			>
				<div class="mx-auto w-full max-w-[1600px]">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
</Sidebar.Provider>
