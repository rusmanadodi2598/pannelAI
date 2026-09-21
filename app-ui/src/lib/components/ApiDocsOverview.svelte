<script lang="ts">
	// The two facts a client needs before its first call: where to send it, and what proves the caller.
	// (docs/SPEC-UI/001-SPEC-UI.md §6.12.)
	//
	// Both come from the document. The base URL is the document's own servers block, and a document that
	// declares none says so instead of borrowing the address the panel happens to reach the gateway on:
	// that address is the panel server's configuration, and a client pointed at it would be talking to
	// the panel rather than to the gateway.
	//
	// The scheme rows are the document's securitySchemes, each paired with the count of operations that
	// declare it, so the reader can tell a scheme in use from one the document still defines.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import {
		credentialLabel,
		schemePlacement,
		type SchemeUse
	} from '$lib/schemas/openapi-credentials';
	import type { OpenAPIDocument } from '$lib/schemas/openapi';
	import { API_DOCS_COPY as copy } from '$lib/strings/api-docs';

	let { doc, uses, publicCount }: { doc: OpenAPIDocument; uses: SchemeUse[]; publicCount: number } =
		$props();

	const servers = $derived(doc.servers ?? []);
</script>

<section class="flex flex-col gap-4">
	<h2 class="text-base font-medium">{copy.baseUrl.heading}</h2>

	{#if servers.length === 0}
		<p class="text-sm text-[var(--color-text-muted)]">{copy.baseUrl.absent}</p>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each servers as server (server.url)}
				<li
					class="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2"
				>
					<code class="font-mono text-sm break-all">{server.url}</code>
					{#if server.description}
						<span class="text-xs text-[var(--color-text-muted)]">{server.description}</span>
					{/if}
					<span class="ms-auto">
						<CopyButton value={server.url} />
					</span>
				</li>
			{/each}
		</ul>
	{/if}

	<h2 class="text-base font-medium">{copy.credentials.heading}</h2>

	<!-- The document's own sentence about its two planes, rendered verbatim: it is the contract's
	     explanation of where each credential goes, and rewriting it here would be the second copy
	     §6.12 rules out. -->
	{#if doc.info.description}
		<p class="max-w-3xl text-sm text-[var(--color-text-muted)]">{doc.info.description}</p>
	{/if}

	{#if uses.length === 0}
		<p class="text-sm text-[var(--color-text-muted)]">{copy.credentials.absent}</p>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each uses as use (use.name)}
				<li
					class="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 py-2"
				>
					<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
						<!-- The panel's word for the scheme, then the document's own name for it, so the
						     labels used in the tables below are anchored to something the contract declares. -->
						<span class="text-sm font-medium">{credentialLabel([use.name])}</span>
						<code class="font-mono text-xs text-[var(--color-text-muted)]">{use.name}</code>
						{#if use.scheme}
							<code class="font-mono text-xs text-[var(--color-text-muted)]"
								>{schemePlacement(use.scheme)}</code
							>
						{/if}
						<span class="ms-auto text-xs text-[var(--color-text-muted)]"
							>{copy.credentials.usage(use.operations)}</span
						>
					</div>

					{#if use.scheme?.description}
						<p class="text-sm text-[var(--color-text-muted)]">{use.scheme.description}</p>
					{:else if !use.scheme}
						<p class="text-sm text-[var(--color-text-muted)]">{copy.credentials.undefined}</p>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if publicCount > 0}
		<p class="text-sm text-[var(--color-text-muted)]">
			{copy.credentials.publicOperations(publicCount)}
		</p>
	{/if}
</section>
