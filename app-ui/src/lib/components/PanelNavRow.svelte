<script lang="ts">
	// One sidebar navigation row.
	//
	// Three shapes come out of one node, and which one applies is decided by the data rather than by a
	// flag someone has to remember to set:
	//
	//   A node with children is a disclosure: a button with `aria-expanded`, because a container is not a
	//   route and must not look like one.
	//   A node with an `href` is a link.
	//   A node with neither is inert text plus a Planned chip, which is what makes R-24 structural: there
	//   is nothing to click.
	//
	// DESIGN.md §6: the active row carries the identity motif, a 3px accent marker on the leading edge,
	// always paired with the label so position is never carried by colour alone.
	import { resolve } from '$app/paths';
	import { navIcon } from '$lib/icons';
	import { isActiveRoute, type NavNode } from '$lib/navigation';
	import * as Collapsible from '$lib/primitives/collapsible/index.js';
	import * as Sidebar from '$lib/primitives/sidebar/index.js';
	import { ChevronRight } from '@lucide/svelte';

	type Props = {
		item: NavNode;
		pathname: string;
		expanded: boolean;
		ontoggle: () => void;
		onnavigate: () => void;
	};

	let { item, pathname, expanded, ontoggle, onnavigate }: Props = $props();

	const icon = $derived(navIcon(item.key));

	// `resolve()` only accepts a real RouteId, so the value is narrowed to a local before the call. Both
	// call sites below sit inside an `{#if}` that proves the type, which is why the helper shape is not
	// needed here.
	const href = $derived(item.href);
	const active = $derived(href ? isActiveRoute(pathname, href) : false);
</script>

<Sidebar.MenuItem>
	{#if item.children}
		<Collapsible.Root open={expanded} onOpenChange={ontoggle}>
			<Collapsible.Trigger>
				{#snippet child({ props })}
					<Sidebar.MenuButton {...props} tooltipContent={item.label} class="min-h-11">
						{#if icon}
							<icon.icon aria-hidden="true" />
						{/if}
						<span>{item.label}</span>
						<ChevronRight
							aria-hidden="true"
							class="ml-auto size-4 transition-transform data-[open=true]:rotate-90 group-data-[collapsible=icon]:hidden"
							data-open={expanded}
						/>
					</Sidebar.MenuButton>
				{/snippet}
			</Collapsible.Trigger>

			<Collapsible.Content>
				<Sidebar.MenuSub class="ml-3 border-s border-[var(--color-sidebar-border)]">
					{#each item.children as child (child.key)}
						{@const childIcon = navIcon(child.key)}
						<Sidebar.MenuSubItem>
							{#if child.href}
								{@const childHref = child.href}
								<Sidebar.MenuSubButton
									href={resolve(childHref)}
									isActive={isActiveRoute(pathname, childHref)}
									onclick={onnavigate}
									class="min-h-11"
								>
									{#if childIcon}
										<childIcon.icon aria-hidden="true" />
									{/if}
									<span>{child.label}</span>
								</Sidebar.MenuSubButton>
							{:else}
								<div
									class="flex min-h-11 min-w-0 items-center gap-2 rounded-[var(--radius-sm)] px-2 text-sm text-[var(--color-text-muted)]"
									title={`${child.label} is planned`}
								>
									{#if childIcon}
										<childIcon.icon aria-hidden="true" class="size-4 shrink-0" />
									{/if}
									<span class="truncate">{child.label}</span>
									<span
										class="ms-auto rounded-[var(--radius-sm)] border border-[var(--color-border)] px-1.5 py-0.5 text-[10px]"
										>Planned</span
									>
								</div>
							{/if}
						</Sidebar.MenuSubItem>
					{/each}
				</Sidebar.MenuSub>
			</Collapsible.Content>
		</Collapsible.Root>
	{:else if href}
		<Sidebar.MenuButton
			isActive={active}
			tooltipContent={item.label}
			class="min-h-11 data-[active=true]:font-medium"
		>
			{#snippet child({ props })}
				<a
					{...props}
					href={resolve(href)}
					aria-current={active ? 'page' : undefined}
					onclick={onnavigate}
				>
					{#if active}
						<span
							aria-hidden="true"
							class="absolute inset-y-1.5 start-0 w-[3px] rounded-[var(--radius-full)] bg-[var(--color-accent)]"
						></span>
					{/if}
					{#if icon}
						<icon.icon aria-hidden="true" />
					{/if}
					<span>{item.label}</span>
				</a>
			{/snippet}
		</Sidebar.MenuButton>
	{:else}
		<div
			class="flex min-h-11 w-full items-center gap-2 rounded-[var(--radius-sm)] p-2 text-left text-sm text-[var(--color-text-muted)]"
			title={`${item.label} is planned`}
		>
			{#if icon}
				<icon.icon aria-hidden="true" class="size-4 shrink-0" />
			{/if}
			<span class="truncate">{item.label}</span>
			<span
				class="ms-auto rounded-[var(--radius-sm)] border border-[var(--color-border)] px-1.5 py-0.5 text-[10px] group-data-[collapsible=icon]:hidden"
				>Planned</span
			>
		</div>
	{/if}
</Sidebar.MenuItem>
