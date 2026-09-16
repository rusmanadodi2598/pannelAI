<script lang="ts">
	// Panel shell: sidebar, header, and content frame.
	//
	// Navigation follows the owner KEEP list in docs/SPEC-UI/001-SPEC-UI.md §5.2. An item whose screen
	// is not built yet is not a link: it renders with a Planned chip, because a nav item pointing at a
	// missing route is exactly what R-24 forbids.
	import { page } from '$app/state';
	import { navIcon } from '$lib/icons';
	import { session } from '$lib/stores/session.svelte';
	import { theme } from '$lib/stores/theme.svelte';
	import { Moon, Sun } from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	type NavItem = { key: string; label: string; href: string; built: boolean };

	const NAV: NavItem[] = [
		{ key: 'endpoint-keys', label: 'Endpoint & Key', href: '/endpoint-keys', built: true },
		{ key: 'providers', label: 'Providers', href: '/providers', built: false },
		{ key: 'combos', label: 'Combo & Vision Adapter', href: '/combos', built: false },
		{ key: 'usage', label: 'Usage', href: '/usage', built: false },
		{ key: 'quota', label: 'Quota Tracker', href: '/quota', built: false },
		{ key: 'token-saver', label: 'Token Saver', href: '/token-saver', built: false },
		{ key: 'proxies', label: 'Proxy Pools', href: '/proxy-pools', built: false },
		{ key: 'skills', label: 'Skills', href: '/skills', built: false },
		{ key: 'logs', label: 'Logs', href: '/logs', built: false },
		{ key: 'api-docs', label: 'API Docs', href: '/api-docs', built: false },
		{ key: 'settings', label: 'Settings', href: '/settings', built: true }
	];

	let { children }: { children: Snippet } = $props();

	let copied = $state(false);
	const apiBase = $derived(
		typeof location === 'undefined' ? '/api/v1' : `${location.origin}/api/v1`
	);

	const current = $derived(page.url.pathname);

	function isActive(href: string): boolean {
		return current === href || current.startsWith(`${href}/`);
	}

	async function copyApiBase(): Promise<void> {
		await navigator.clipboard.writeText(apiBase);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<div class="flex min-h-screen flex-col lg:flex-row">
	<aside
		class="flex shrink-0 flex-col gap-1 border-b border-[var(--color-border)] bg-[var(--color-surface-2)] p-3 lg:w-64 lg:border-b-0 lg:border-r"
	>
		<p class="px-2 py-3 text-sm font-semibold tracking-tight">KENTANG TECH pannelAI</p>

		<nav aria-label="Panel sections" class="flex flex-col gap-0.5">
			{#each NAV as item (item.key)}
				{@const icon = navIcon(item.key)}
				{#if item.built}
					<a
						href={item.href}
						aria-current={isActive(item.href) ? 'page' : undefined}
						class="flex min-h-11 items-center gap-2.5 rounded-[var(--radius-sm)] px-2 text-sm transition-colors
							{isActive(item.href)
							? 'bg-[var(--color-surface-3)] font-medium'
							: 'hover:bg-[var(--color-surface-3)]'}"
					>
						{#if icon}
							<icon.icon
								class="size-4 shrink-0 text-[var(--color-text-muted)]"
								aria-hidden="true"
							/>
						{/if}
						<span>{item.label}</span>
					</a>
				{:else}
					<span
						class="flex min-h-11 items-center gap-2.5 px-2 text-sm text-[var(--color-text-muted)]"
					>
						{#if icon}
							<icon.icon class="size-4 shrink-0" aria-hidden="true" />
						{/if}
						<span>{item.label}</span>
						<span
							class="ml-auto rounded-[var(--radius-sm)] border border-[var(--color-border)] px-1.5 py-0.5 text-[11px]"
							>Planned</span
						>
					</span>
				{/if}
			{/each}
		</nav>
	</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		<header
			class="flex flex-wrap items-center gap-3 border-b border-[var(--color-border)] px-4 py-3"
		>
			<div class="flex items-center gap-2 text-xs text-[var(--color-text-muted)]">
				<span class="font-medium text-[var(--color-text)]">API base</span>
				<code class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-1.5 py-1"
					>{apiBase}</code
				>
				<button
					type="button"
					class="rounded-[var(--radius-sm)] px-1.5 py-1 underline hover:text-[var(--color-text)]"
					onclick={copyApiBase}
				>
					{copied ? 'Copied' : 'Copy'}
				</button>
			</div>

			<div class="ml-auto flex items-center gap-2">
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
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
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
					onclick={() => session.signOut()}
				>
					Sign out
				</button>
			</div>
		</header>

		<main class="min-w-0 flex-1 p-4 lg:p-6">
			{@render children()}
		</main>
	</div>
</div>
