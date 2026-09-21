<script lang="ts">
	// The entry skill's install line (docs/SPEC-UI/001-SPEC-UI.md §6.10).
	//
	// This is the page's focal point, because it is the one thing an operator comes here to take: the line
	// that teaches an AI client the whole gateway. The capability rows below are for the narrower case.
	//
	// §6.10's rule applies here as much as to a row: a copy control that copies a broken link is not
	// shipped. So the line and its control appear only once the entry skill's own source has answered, and
	// the block states the cause in their place otherwise.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { installLine, type SourceProbe } from '$lib/schemas/skill-source';
	import type { Skill } from '$lib/schemas/skill';
	import { SKILLS_COPY as copy, causeSentence } from '$lib/strings/skills';

	let { skill, probe }: { skill: Skill; probe: SourceProbe | undefined } = $props();

	const available = $derived(probe?.state === 'available');
	const state = $derived(
		available ? copy.sources.available : probe ? copy.sources.unavailable : copy.sources.checkingRow
	);
</script>

<section
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-4"
>
	<div class="flex flex-col gap-1">
		<h2 class="text-sm font-semibold">{copy.entry.heading}</h2>
		<p class="text-xs text-[var(--color-text-muted)]">{copy.entry.intro}</p>
	</div>

	<div class="flex flex-col gap-1">
		<div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
			<span class="text-sm font-medium">{skill.name}</span>
			<span class="text-xs text-[var(--color-text-muted)]">{state}</span>
		</div>
		<p class="text-sm text-[var(--color-text-muted)]">{skill.description}</p>
	</div>

	{#if available}
		<div class="flex flex-wrap items-center gap-3">
			<CopyButton value={installLine(skill)} label={copy.sources.copyInstall} />
		</div>

		<!-- The line stays on screen beside the control, so a refused clipboard write still leaves the
		     operator something to select. -->
		<pre
			class="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap"><code
				>{installLine(skill)}</code
			></pre>

		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the blob URL is the source host's own absolute URL, never a route of this app -->
		<a class="text-xs underline" href={skill.blob_url} target="_blank" rel="noreferrer"
			>{copy.sources.readOnGitHub}</a
		>
	{:else if probe?.state === 'unavailable'}
		<p class="text-xs text-[var(--color-text-muted)]">{causeSentence(probe.cause)}</p>
		<code class="font-mono text-xs break-all text-[var(--color-text-muted)]">{skill.raw_url}</code>
	{/if}
</section>
