<script lang="ts">
	// The error code table (docs/SPEC-UI/001-SPEC-UI.md §6.12).
	//
	// The codes, their statuses, and the meaning sentences all come from the document: the code map is
	// its `x-contract.planes` block, and a meaning is the description of the response whose body is that
	// plane's error envelope. Nothing is restated from SPEC-API §8, which is what §6.12 requires.
	//
	// A document without the block renders the reason it is absent rather than an empty table, and a code
	// the document declares without describing says so in the cell, because a blank cell reads as a
	// rendering fault instead of as a fact about the document.
	import { NO_DESCRIPTION, type ErrorPlane } from '$lib/schemas/openapi-errors';
	import { API_DOCS_COPY as copy } from '$lib/strings/api-docs';

	let { planes }: { planes: ErrorPlane[] } = $props();
</script>

<section class="flex flex-col gap-4">
	<h2 class="text-base font-medium">{copy.errors.heading}</h2>

	{#if planes.length === 0}
		<p class="max-w-3xl text-sm text-[var(--color-text-muted)]">{copy.errors.absent}</p>
	{:else}
		{#each planes as plane (plane.key)}
			<div class="flex flex-col gap-2">
				<h3 class="text-sm font-semibold">{plane.label}</h3>

				{#if plane.envelope}
					<p class="text-sm text-[var(--color-text-muted)]">
						{copy.errors.envelope(plane.envelope)}
					</p>
				{/if}

				<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
					<table class="w-full min-w-[36rem] border-collapse text-sm">
						<caption class="sr-only">Error codes in the {plane.label} plane</caption>
						<thead class="bg-[var(--color-surface-2)] text-left">
							<tr>
								<th scope="col" class="px-3 py-2 font-medium">Code</th>
								<th scope="col" class="px-3 py-2 font-medium">HTTP</th>
								<th scope="col" class="px-3 py-2 font-medium">Meaning</th>
							</tr>
						</thead>
						<tbody>
							{#each plane.rows as row (row.code)}
								<tr class="border-t border-[var(--color-border)] align-top">
									<td class="px-3 py-2 font-mono text-xs">{row.code}</td>
									<td class="px-3 py-2 tabular-nums">{row.status}</td>
									<td class="px-3 py-2">
										{#if row.description}
											{row.description}
										{:else}
											<span class="text-[var(--color-text-muted)]">{NO_DESCRIPTION}</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/each}
	{/if}
</section>
