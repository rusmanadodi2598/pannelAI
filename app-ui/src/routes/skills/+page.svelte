<script lang="ts">
	// Skills (docs/SPEC-UI/001-SPEC-UI.md §6.10).
	//
	// The screen has one job: hand the operator the line to paste into an AI client, per capability. The
	// catalog comes from `GET /api/v1/skills` and never carries a document body, so every line is composed
	// from the row's own address.
	//
	// The second job is honesty. §6.10 forbids a copy control that copies a broken link, and the catalog
	// cannot say whether a document exists, so each address is asked and its answer decides what renders.
	// The check runs on load and again on request, because the fact it reports changes when a document is
	// published rather than when the panel changes.
	import { onMount } from 'svelte';
	import SkillCard from '$lib/components/SkillCard.svelte';
	import SkillEntryBlock from '$lib/components/SkillEntryBlock.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { fetchSkillCatalog, probeSkillSources, SKILLS_PATH } from '$lib/api/skills';
	import { orderSkills, type SourceProbe } from '$lib/schemas/skill-source';
	import type { SkillCatalog } from '$lib/schemas/skill';
	import { SKILLS_COPY as copy } from '$lib/strings/skills';

	let catalog = $state<SkillCatalog | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let probes = $state<Record<string, SourceProbe>>({});
	let probing = $state(false);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		error = null;

		const result = await fetchSkillCatalog();

		if (!result.ok) {
			loading = false;
			error = result.error.message;
			return;
		}

		catalog = result.data;
		loading = false;

		await checkSources(result.data);
	}

	// `probes` is emptied first, so a row with no entry in it is one the check has not answered yet
	// rather than one the panel has an old answer for.
	async function checkSources(next: SkillCatalog): Promise<void> {
		probing = true;
		probes = {};
		probes = await probeSkillSources(orderSkills(next.data));
		probing = false;
	}

	// Reads the catalog out of state before awaiting, so the control never hands a null to the probe.
	async function checkAgain(): Promise<void> {
		const next = catalog;
		if (next) await checkSources(next);
	}

	const skills = $derived(catalog ? orderSkills(catalog.data) : []);
	const entry = $derived(skills.find((skill) => skill.entry) ?? null);
	const capabilities = $derived(skills.filter((skill) => !skill.entry));
	// Counts the published rows rather than the answered ones, because every probe answers: the sentence
	// reports what the check found, not that it ran.
	const published = $derived(
		skills.filter((skill) => probes[skill.id]?.state === 'available').length
	);
</script>

<section class="flex max-w-4xl flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>
		<p class="text-sm text-[var(--color-text-muted)]">{copy.subtitle}</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title={copy.catalog.loading} />
	{:else if error}
		<StateMessage kind="error" title={copy.catalog.errorTitle} description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}
					>{copy.catalog.retry}</button
				>
			{/snippet}
		</StateMessage>
	{:else if catalog && skills.length === 0}
		<StateMessage
			kind="empty"
			title={copy.catalog.emptyTitle}
			description={copy.catalog.emptyDescription}
		/>
	{:else if catalog}
		<!-- What was read, stated as facts about the catalog rather than as a claim about the documents. -->
		<div
			class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm text-[var(--color-text-muted)]"
		>
			<span>{copy.catalog.readFrom(SKILLS_PATH)}</span>
			<span>{copy.catalog.rows(skills.length)}</span>
		</div>

		<!-- The control sits above what it re-reads, and the sentence beside it is the measured count
		     rather than a claim that every source is fine. -->
		<div class="flex flex-wrap items-center gap-3">
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				{probing
					? copy.sources.checking(skills.length)
					: copy.sources.summary(published, skills.length)}
			</p>
			<button
				type="button"
				class="min-h-11 text-sm underline disabled:opacity-50"
				disabled={probing}
				onclick={() => void checkAgain()}>{copy.sources.checkAgain}</button
			>
		</div>

		{#if entry}
			<SkillEntryBlock skill={entry} probe={probes[entry.id]} />
		{:else}
			<p class="text-sm text-[var(--color-text-muted)]">{copy.entry.absent}</p>
		{/if}

		{#if capabilities.length > 0}
			<div class="flex flex-col gap-3">
				<div class="flex flex-col gap-1">
					<h2 class="text-base font-medium">{copy.sources.heading}</h2>
					<p class="text-sm text-[var(--color-text-muted)]">{copy.sources.intro}</p>
				</div>

				<ul class="flex flex-col gap-3">
					{#each capabilities as skill (skill.id)}
						<SkillCard {skill} probe={probes[skill.id]} />
					{/each}
				</ul>
			</div>
		{/if}
	{/if}
</section>
