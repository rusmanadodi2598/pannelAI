<script lang="ts">
	// Panel sidebar body: the navigation tree, rendered from src/lib/navigation.ts.
	//
	// The tree is data, so this component decides only how a group looks and where the row component goes.
	// The row itself is PanelNavRow, which owns the three shapes a node can take. Splitting them keeps
	// each file readable and under the project's size limit.
	//
	// The one state that lives here is which disclosures are open, because that is per sidebar rather than
	// per row: a container starts open when a child is the current route, so a reload on a Media kind page
	// does not hide the row the operator is looking at.
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import PanelNavRow from '$lib/components/PanelNavRow.svelte';
	import { navPath } from '$lib/nav-path';
	import { isActiveRoute, NAV_GROUPS, type NavNode } from '$lib/navigation';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';

	// The primitive layer keeps the mobile drawer's state in its own context, so a row click has to close
	// it explicitly. Without this the drawer stays open over the page the operator just picked.
	const sidebar = Sidebar.useSidebar();

	const current = $derived(page.url.pathname);

	let expanded = $state<Record<string, boolean>>({});

	function isExpanded(node: NavNode): boolean {
		const chosen = expanded[node.key];
		if (chosen !== undefined) return chosen;

		// Default is closed. It opens only when a child is the current route, and the check goes through
		// `navPath` so a parameterised child (a Media kind) counts the same as a static one.
		return (
			node.children?.some((child) => {
				const path = navPath(child);
				return path !== undefined && isActiveRoute(current, path);
			}) ?? false
		);
	}

	function toggle(node: NavNode): void {
		expanded[node.key] = !isExpanded(node);
	}

	function handleNavigate(): void {
		if (sidebar.isMobile) sidebar.setOpenMobile(false);
	}
</script>

<Sidebar.Root collapsible="icon" class="border-e border-[var(--color-sidebar-border)]">
	<Sidebar.Header class="border-b border-[var(--color-sidebar-border)]">
		<a
			href={resolve('/endpoint-keys')}
			class="flex h-11 items-center gap-2.5 px-1"
			aria-label="pannelAI, go to endpoint keys"
			onclick={handleNavigate}
		>
			<img
				src="/logo-mark@128.png"
				alt=""
				width="28"
				height="28"
				class="size-7 shrink-0 rounded-full ring-1 ring-[var(--color-border)] ring-offset-1 ring-offset-[var(--color-sidebar)]"
			/>
			<span class="grid min-w-0 leading-tight group-data-[collapsible=icon]:hidden">
				<span class="truncate text-sm font-semibold tracking-tight">pannelAI</span>
				<span class="truncate text-[11px] text-[var(--color-text-muted)]">KENTANG TECH gateway</span
				>
			</span>
		</a>
	</Sidebar.Header>

	<Sidebar.Content class="px-2 py-2">
		<!-- A `nav` landmark with a name, so a screen-reader user can jump straight to the panel's
		     navigation instead of walking the header and the logo first. The primitive layer renders a
		     plain div for the sidebar container, so this is the panel's job rather than shadcn's. Both
		     render paths (the desktop container and the mobile Sheet) draw the same children, so the
		     landmark exists at every breakpoint. -->
		<nav aria-label="Panel navigation" class="flex flex-col gap-2">
			{#each NAV_GROUPS as group (group.key)}
				<Sidebar.Group class="py-1">
					<Sidebar.GroupLabel
						class="h-7 px-2 text-[11px] font-semibold tracking-[0.06em] text-[var(--color-text-muted)] uppercase group-data-[collapsible=icon]:hidden"
					>
						{group.label}
					</Sidebar.GroupLabel>

					<Sidebar.GroupContent>
						<Sidebar.Menu>
							{#each group.items as item (item.key)}
								<PanelNavRow
									{item}
									pathname={current}
									expanded={isExpanded(item)}
									ontoggle={() => toggle(item)}
									onnavigate={handleNavigate}
								/>
							{/each}
						</Sidebar.Menu>
					</Sidebar.GroupContent>
				</Sidebar.Group>
			{/each}
		</nav>
	</Sidebar.Content>

	<Sidebar.Rail />
</Sidebar.Root>
