<script lang="ts">
	// API Docs (docs/SPEC-UI/001-SPEC-UI.md §6.12).
	//
	// The screen renders the contract the gateway serves, so it has no content of its own beyond labels:
	// the base URL, the credentials, the catalog, the examples, and the error codes are all read from
	// `GET /api/v1/openapi.json`. §6.12 forbids a second, hand-written copy of the contract, and the way
	// to obey that is to have exactly one source and to state an absence when the document is silent.
	//
	// The document is also the phase answer: it lists what the router registers, and app-serv pins the
	// two against each other in both directions (TestOpenAPICoversEveryRegisteredRoute), so a group on
	// this screen is a route that exists rather than a route that is planned. The screen therefore states
	// the document's version and the address it was read from instead of a per-group phase.
	import { onMount } from 'svelte';
	import ApiDocsErrorCodes from '$lib/components/ApiDocsErrorCodes.svelte';
	import ApiDocsGroup from '$lib/components/ApiDocsGroup.svelte';
	import ApiDocsOverview from '$lib/components/ApiDocsOverview.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { fetchOpenAPIDocument, OPENAPI_PATH } from '$lib/api/docs';
	import { documentGroups, groupAnchor } from '$lib/schemas/openapi-catalog';
	import { publicOperationCount, schemeUses } from '$lib/schemas/openapi-credentials';
	import { errorPlanes } from '$lib/schemas/openapi-errors';
	import type { OpenAPIDocument } from '$lib/schemas/openapi';
	import { API_DOCS_COPY as copy } from '$lib/strings/api-docs';

	let doc = $state<OpenAPIDocument | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		error = null;

		const result = await fetchOpenAPIDocument();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		doc = result.data;
	}

	const groups = $derived(doc ? documentGroups(doc) : []);
	const uses = $derived(doc ? schemeUses(doc) : []);
	const publicCount = $derived(doc ? publicOperationCount(doc) : 0);
	const planes = $derived(doc ? errorPlanes(doc) : []);
</script>

<section class="flex flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>
		<p class="text-sm text-[var(--color-text-muted)]">{copy.subtitle}</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title={copy.document.loading} />
	{:else if error}
		<StateMessage kind="error" title={copy.document.errorTitle} description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}
					>{copy.document.retry}</button
				>
			{/snippet}
		</StateMessage>
	{:else if doc}
		<!-- What was read, stated as facts about the document rather than as a claim about the gateway. -->
		<div
			class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm text-[var(--color-text-muted)]"
		>
			<span class="text-[var(--color-text)]">{doc.info.title}</span>
			<span>{copy.document.version(doc.info.version)}</span>
			<code class="font-mono text-xs">OpenAPI {doc.openapi}</code>
			<span>{copy.document.readFrom(OPENAPI_PATH)}</span>
		</div>

		<ApiDocsOverview {doc} {uses} {publicCount} />

		{#if groups.length === 0}
			<StateMessage
				kind="empty"
				title={copy.document.emptyTitle}
				description={copy.document.emptyDescription}
			/>
		{:else}
			<div class="flex flex-col gap-6">
				<div class="flex flex-col gap-2">
					<h2 class="text-base font-medium">{copy.catalog.heading}</h2>
					<p class="text-sm text-[var(--color-text-muted)]">{copy.catalog.intro}</p>

					<!-- A jump list, because a reference with twenty groups is read by looking one up. -->
					<nav aria-label={copy.catalog.indexLabel} class="flex flex-wrap gap-x-4 gap-y-1 pt-1">
						{#each groups as group (group.name)}
							<a class="text-sm underline" href={`#${groupAnchor(group.name)}`}>
								{group.name}
								<span class="text-xs text-[var(--color-text-muted)]"
									>({group.operations.length})</span
								>
							</a>
						{/each}
					</nav>
				</div>

				{#each groups as group (group.name)}
					<ApiDocsGroup {group} {doc} />
				{/each}
			</div>
		{/if}

		<ApiDocsErrorCodes {planes} />
	{/if}
</section>
