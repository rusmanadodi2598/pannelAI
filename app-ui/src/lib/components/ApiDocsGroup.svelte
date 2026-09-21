<script lang="ts">
	// One tag's section of the catalog (docs/SPEC-UI/001-SPEC-UI.md §6.12).
	//
	// The table carries one row per operation the document lists under this tag, and the credential
	// column is per row rather than per group because a group can mix them: the Auth, OAuth, and System
	// groups each hold a public operation beside a session-gated one.
	//
	// One example call per group is what §6.12 asks for, and it is composed from the example operation's
	// own method, path, and credential. Nothing about the call is typed by hand, so the example cannot
	// describe a route the document does not list.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { curlExample, credentialLabel } from '$lib/schemas/openapi-credentials';
	import { groupAnchor, groupExample, type ApiGroup } from '$lib/schemas/openapi-catalog';
	import type { OpenAPIDocument } from '$lib/schemas/openapi';
	import { API_DOCS_COPY as copy } from '$lib/strings/api-docs';

	let { group, doc }: { group: ApiGroup; doc: OpenAPIDocument } = $props();

	const anchor = $derived(groupAnchor(group.name));
	const example = $derived(groupExample(group));
	const curl = $derived(example ? curlExample(example, doc) : '');
</script>

<section class="flex flex-col gap-3">
	<h3 id={anchor} class="scroll-mt-4 text-sm font-semibold">
		{group.name}
		<span class="ms-2 font-normal text-[var(--color-text-muted)]"
			>{copy.catalog.operations(group.operations.length)}</span
		>
	</h3>

	<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
		<table class="w-full min-w-[40rem] border-collapse text-sm">
			<caption class="sr-only"
				>Operations in {group.name}, with the credential each one requires</caption
			>
			<thead class="bg-[var(--color-surface-2)] text-left">
				<tr>
					<th scope="col" class="px-3 py-2 font-medium">{copy.catalog.columns.method}</th>
					<th scope="col" class="px-3 py-2 font-medium">{copy.catalog.columns.path}</th>
					<th scope="col" class="px-3 py-2 font-medium">{copy.catalog.columns.summary}</th>
					<th scope="col" class="px-3 py-2 font-medium">{copy.catalog.columns.credential}</th>
				</tr>
			</thead>
			<tbody>
				{#each group.operations as operation (`${operation.method} ${operation.path}`)}
					<tr class="border-t border-[var(--color-border)] align-top">
						<td class="px-3 py-2 font-mono text-xs uppercase">{operation.method}</td>
						<td class="px-3 py-2 font-mono text-xs break-all">{operation.path}</td>
						<td class="px-3 py-2">
							{#if operation.summary}
								{operation.summary}
							{:else}
								<span class="text-[var(--color-text-muted)]">{copy.catalog.noSummary}</span>
							{/if}
						</td>
						<td class="px-3 py-2">{credentialLabel(operation.schemes)}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if curl}
		<div class="flex flex-col gap-2">
			<div class="flex flex-wrap items-center gap-3">
				<span class="text-xs font-medium text-[var(--color-text-muted)]"
					>{copy.catalog.example}</span
				>
				<CopyButton value={curl} />
			</div>
			<!-- The example stays on screen beside the control, so a refused clipboard write still leaves
			     the operator something to select. -->
			<pre
				class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 font-mono text-xs"><code
					>{curl}</code
				></pre>
		</div>
	{/if}
</section>
