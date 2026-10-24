<script lang="ts">
	// The provider detail's Overview tab (docs/SPEC-UI/001-SPEC-UI.md §6.3, "Detail" / "Tabs").
	//
	// The facts are grouped into three cards by the question each one answers: how the gateway reaches the
	// provider (transport), who may use it and whether it routes (access), and what it holds (models and
	// endpoints). A flat list of eleven label/value pairs gave every fact the same weight, so the one that
	// says the provider cannot answer (no endpoint, or a connector that is not translated) read like the
	// name of a category.
	//
	// The status line above the cards carries the one thing an operator must see first: whether the provider
	// routes and whether it has an endpoint. The colour is a marker on the leading edge, the motif DESIGN.md
	// §6 reserves for health, and the words carry the same meaning, so colour is never the only signal.
	//
	// An empty transport value renders as "Not set" rather than a blank cell, so a missing value cannot be
	// mistaken for a value the panel failed to read. A timeout of zero is stated as "No timeout", because a
	// bare "0 ms" reads as a configured limit of nothing.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { AUTH_TYPE_LABELS } from '$lib/schemas/endpoint';
	import { statusSummaryText, type ProviderDetail } from '$lib/schemas/provider';

	let { provider }: { provider: ProviderDetail } = $props();

	const ROUTABILITY_LABELS: Record<string, string> = {
		native: 'Native',
		connector: 'Connector'
	};

	const shown = (value: string): string => (value.trim() === '' ? 'Not set' : value);

	const timeout = $derived(provider.timeout_ms === 0 ? 'No timeout' : `${provider.timeout_ms} ms`);

	// The status line states the one thing an operator acts on first, in the words that match the reason.
	// A connector is not translated, so it cannot answer at all; a failing endpoint is broken; a provider
	// with no endpoint has nothing to call yet, which is the state a fresh entry starts in; anything else
	// routes. The tone picks the marker colour, and the sentence carries the same meaning on its own.
	type Tone = 'ok' | 'warn' | 'idle';
	const verdict = $derived.by((): { tone: Tone; text: string } => {
		if (provider.routability === 'connector') return { tone: 'warn', text: 'Not routable' };
		if (provider.status_summary.error > 0) return { tone: 'warn', text: 'Endpoints failing' };
		if (provider.status_summary.total === 0) return { tone: 'idle', text: 'No endpoint yet' };
		return { tone: 'ok', text: 'Routing' };
	});

	const TONE_MARKER: Record<Tone, string> = {
		ok: 'border-[var(--color-ok)]',
		warn: 'border-[var(--color-warn)]',
		idle: 'border-[var(--color-border)]'
	};
</script>

<div class="flex flex-col gap-4">
	<p
		class={`flex items-center gap-3 border-l-[3px] py-1 pl-3 text-sm ${TONE_MARKER[verdict.tone]}`}
		role="status"
	>
		<span class="font-medium">{verdict.text}</span>
		<span class="text-[var(--color-text-muted)]">
			{ROUTABILITY_LABELS[provider.routability] ?? provider.routability} routability{#if provider.status_summary.total > 0},
				{statusSummaryText(provider.status_summary)}{/if}
		</span>
	</p>

	{#if provider.routability === 'connector'}
		<p class="text-sm text-[var(--color-warn)]">
			This provider's protocol isn't translated, so its endpoints won't answer.
		</p>
	{/if}

	<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		<section
			class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
			aria-labelledby="facts-transport"
		>
			<header class="flex min-h-9 items-center justify-between gap-2">
				<h3 id="facts-transport" class="text-sm font-medium">Transport</h3>
				{#if provider.base_url.trim() !== ''}
					<!-- The copy sits in the heading row, beside the value it copies, so it reads as an action on
					     the card rather than as part of the value's text. -->
					<CopyButton value={provider.base_url} label="Copy base URL" iconOnly touchTarget />
				{/if}
			</header>
			<dl class="flex flex-col gap-3">
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Base URL</dt>
					<dd class="break-all text-sm">{shown(provider.base_url)}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Format</dt>
					<dd class="text-sm">{shown(provider.format)}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">URL suffix</dt>
					<dd class="break-all text-sm">{shown(provider.url_suffix)}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Validate URL</dt>
					<dd class="break-all text-sm">{shown(provider.validate_url)}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Timeout</dt>
					<dd class="text-sm tabular-nums">{timeout}</dd>
				</div>
			</dl>
		</section>

		<section
			class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
			aria-labelledby="facts-access"
		>
			<header class="flex min-h-9 items-center justify-between gap-2">
				<h3 id="facts-access" class="text-sm font-medium">Access</h3>
			</header>
			<dl class="flex flex-col gap-3">
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Provider id</dt>
					<dd class="break-all text-sm">{provider.id}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Category</dt>
					<dd class="text-sm">{provider.category}</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Auth</dt>
					<dd class="text-sm">
						{AUTH_TYPE_LABELS[provider.auth_type] ?? provider.auth_type}
						{#if provider.auth_modes.length > 0}
							<span class="text-[var(--color-text-muted)]">({provider.auth_modes.join(', ')})</span>
						{/if}
					</dd>
				</div>
				{#if provider.website}
					<div class="flex flex-col gap-0.5">
						<dt class="text-xs text-[var(--color-text-muted)]">Website</dt>
						<dd class="break-all text-sm">
							<!-- `rel="external"` is what tells the lint rule this href is not a panel route: `resolve`
							     refuses a non-absolute pathname, and a registry website is an arbitrary external URL. -->
							<a
								href={provider.website}
								class="inline-flex min-h-11 items-center px-2 underline"
								rel="external noreferrer noopener"
								target="_blank">{provider.website}</a
							>
						</dd>
					</div>
				{/if}
			</dl>
		</section>

		<section
			class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
			aria-labelledby="facts-holdings"
		>
			<header class="flex min-h-9 items-center justify-between gap-2">
				<h3 id="facts-holdings" class="text-sm font-medium">Models and endpoints</h3>
			</header>
			<dl class="flex flex-col gap-3">
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Models</dt>
					<dd class="text-sm">
						{provider.model_count} total, {provider.chat_model_count} routable as chat
					</dd>
				</div>
				<div class="flex flex-col gap-0.5">
					<dt class="text-xs text-[var(--color-text-muted)]">Endpoints</dt>
					<dd
						class={`text-sm ${provider.status_summary.total === 0 ? 'text-[var(--color-warn)]' : ''}`}
					>
						{statusSummaryText(provider.status_summary)}
					</dd>
				</div>
			</dl>
		</section>
	</div>
</div>

{#if provider.deprecated}
	<p
		class="mt-3 rounded-[var(--radius-md)] border border-[var(--color-warn)] px-3 py-2 text-sm"
		role="status"
	>
		<span class="font-medium">This provider is deprecated.</span>
		{#if provider.deprecation_notice}
			{provider.deprecation_notice}
		{:else}
			No reason given. Check the provider's site first.
		{/if}
	</p>
{/if}
