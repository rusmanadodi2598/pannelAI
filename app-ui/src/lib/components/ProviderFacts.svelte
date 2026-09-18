<script lang="ts">
	// The provider detail's fact block (docs/SPEC-UI/001-SPEC-UI.md §6.3, "Header").
	//
	// The transport defaults are shown rather than summarized, because reachability is decided by `format`
	// and `base_url` and not by the display name: a provider whose protocol the data plane cannot translate
	// is configured and always failing, and `routability` is the field that says so before the first
	// request. A `connector` provider therefore carries a warning rather than a neutral value, because the
	// operator's next step differs.
	//
	// An empty transport value renders as "Not set" rather than as an empty cell, so a blank line cannot be
	// mistaken for a value the panel failed to read.
	import { AUTH_TYPE_LABELS } from '$lib/schemas/endpoint';
	import { statusSummaryText, type ProviderDetail } from '$lib/schemas/provider';

	let { provider }: { provider: ProviderDetail } = $props();

	const ROUTABILITY_LABELS: Record<string, string> = {
		native: 'Native',
		connector: 'Connector'
	};

	const shown = (value: string): string => (value.trim() === '' ? 'Not set' : value);
</script>

<dl class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Provider id</dt>
		<dd class="text-sm">{provider.id}</dd>
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
				<span class="text-[var(--color-text-muted)]"> ({provider.auth_modes.join(', ')})</span>
			{/if}
		</dd>
	</div>

	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Routability</dt>
		<dd class="text-sm">
			{ROUTABILITY_LABELS[provider.routability] ?? provider.routability}
			{#if provider.routability === 'connector'}
				<span class="text-[var(--color-warn)]">
					This provider speaks a protocol the data plane does not translate, so an endpoint for it
					will not answer.</span
				>
			{/if}
		</dd>
	</div>

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
		<dd class="text-sm tabular-nums">{provider.timeout_ms} ms</dd>
	</div>

	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Models</dt>
		<dd class="text-sm">
			{provider.model_count} total, {provider.chat_model_count} routable as chat
		</dd>
	</div>

	<div class="flex flex-col gap-0.5">
		<dt class="text-xs text-[var(--color-text-muted)]">Endpoints</dt>
		<dd class="text-sm">{statusSummaryText(provider.status_summary)}</dd>
	</div>

	{#if provider.website}
		<div class="flex flex-col gap-0.5">
			<dt class="text-xs text-[var(--color-text-muted)]">Website</dt>
			<dd class="break-all text-sm">
				<!-- `rel="external"` is what tells the lint rule this href is not a panel route: `resolve`
				     refuses a non-absolute pathname, and a registry website is an arbitrary external URL. -->
				<a
					href={provider.website}
					class="underline"
					rel="external noreferrer noopener"
					target="_blank">{provider.website}</a
				>
			</dd>
		</div>
	{/if}
</dl>

{#if provider.deprecated}
	<p
		class="mt-3 rounded-[var(--radius-md)] border border-[var(--color-warn)] px-3 py-2 text-sm"
		role="status"
	>
		<span class="font-medium">This provider is deprecated.</span>
		{#if provider.deprecation_notice}
			{provider.deprecation_notice}
		{:else}
			The registry marks it deprecated without a notice, so check the provider's own documentation
			before routing to it.
		{/if}
	</p>
{/if}
