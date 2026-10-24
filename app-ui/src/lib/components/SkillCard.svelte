<script lang="ts">
	// One skill row (docs/SPEC-UI/001-SPEC-UI.md §6.10).
	//
	// §6.10 asks for a name, a one-line description, a source link, and a copy control with the install
	// instruction line, and it forbids a copy control that copies a broken link. The last rule is why this
	// row has two shapes rather than one: the control and the link appear only when the source answered,
	// and an unreachable row states the cause in their place. The address itself stays on screen either
	// way, because it names the path that has to be published.
	//
	// The leading marker is DESIGN.md §6's identity motif, carrying this row's availability. It is never
	// alone: the sentence beside it says the same thing in words.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { installLine, type SourceProbe } from '$lib/schemas/skill-source';
	import type { Skill } from '$lib/schemas/skill';
	import { SKILLS_COPY as copy, causeSentence } from '$lib/strings/skills';

	let { skill, probe }: { skill: Skill; probe: SourceProbe | undefined } = $props();

	const available = $derived(probe?.state === 'available');
	const label = $derived(
		available ? copy.sources.available : probe ? copy.sources.unavailable : copy.sources.checkingRow
	);
	const marker = $derived(
		available
			? 'bg-[var(--color-ok)]'
			: probe
				? 'bg-[var(--color-danger)]'
				: 'bg-[var(--color-border)]'
	);
</script>

<li
	class="relative flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 ps-5"
>
	<span
		class="absolute inset-y-4 start-0 w-[3px] rounded-[var(--radius-full)] {marker}"
		aria-hidden="true"
	></span>

	<div class="flex items-start gap-3">
		<div class="flex min-w-0 flex-1 flex-col gap-1">
			<div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
				<h3 class="min-w-0 break-words text-sm font-semibold">{skill.name}</h3>
				{#if skill.endpoint}
					<code class="min-w-0 break-all font-mono text-xs">{skill.endpoint}</code>
				{:else}
					<span class="text-xs text-[var(--color-text-muted)]">{copy.sources.noEndpoint}</span>
				{/if}
			</div>

			<p class="break-words text-sm text-[var(--color-text-muted)]">{skill.description}</p>
		</div>

		{#if available}
			<CopyButton
				value={installLine(skill)}
				label={copy.sources.copyInstall}
				ariaLabel={`Copy install line for ${skill.name}`}
				iconOnly
				glyphOutcome
				touchTarget
			/>
		{/if}
	</div>

	<div class="flex flex-col gap-2 border-t border-[var(--color-border)] pt-3">
		<p class="text-xs font-medium">{label}</p>

		{#if available}
			<span class="text-xs text-[var(--color-text-muted)]">{copy.sources.agentAddress}</span>

			<!-- The address stays on screen even when the clipboard refuses, so the operator can still
			     select it by hand. -->
			<code class="font-mono text-xs break-all text-[var(--color-text-muted)]">{skill.raw_url}</code
			>

			<!-- An action link is a control (DESIGN.md §3.5), and this one routes off-site, so it goes
			     through the primitive with its own target and rel kept. -->
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- the blob URL is the source host's own absolute URL, never a route of this app -->
			<Button
				href={skill.blob_url}
				variant="ghost"
				class="self-start px-2 text-xs underline"
				aria-label={`${copy.sources.readOnGitHub}: ${skill.name}`}
				target="_blank"
				rel="noreferrer">{copy.sources.readOnGitHub}</Button
			>
		{:else if probe?.state === 'unavailable'}
			<p class="text-xs text-[var(--color-text-muted)]">{causeSentence(probe.cause)}</p>
			<code class="font-mono text-xs break-all text-[var(--color-text-muted)]">{skill.raw_url}</code
			>
		{/if}
	</div>
</li>
