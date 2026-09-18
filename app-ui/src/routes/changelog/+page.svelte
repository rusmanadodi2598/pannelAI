<script lang="ts">
	// Changelog (docs/SPEC-UI/001-SPEC-UI.md §6.18).
	//
	// The screen answers one question: what changed, and is this build behind? So the hierarchy is the
	// running version first, then the releases, newest first, each marked against that version.
	//
	// The data source is not decided (SPEC-UI §14 Q11): SPEC-API §7 defines no changelog endpoint. Rather
	// than render a bundled list that would drift from the running gateway, the screen shows what it
	// actually knows plus an honest empty state naming the missing source. When a source lands, it feeds
	// `schemaChangelog` and only the loader below changes.
	//
	// The version header is real: GET /api/v1/version (SPEC-API §7.1) already reports it.
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { fetchSystemInfo } from '$lib/api/system';
	import {
		countNewer,
		releaseMarker,
		sortChangelog,
		type ChangelogEntry
	} from '$lib/schemas/changelog';
	import { CHANGELOG_COPY as copy } from '$lib/strings/changelog';
	import { formatTimestamp } from '$lib/utils/time';
	import { onMount } from 'svelte';

	// Entries come from the panel's release-note source. Empty until one exists, and the empty state says
	// so instead of pretending the gateway has no history.
	let entries = $state<ChangelogEntry[]>([]);
	let runningVersion = $state('');
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		error = null;

		const result = await fetchSystemInfo();
		loading = false;

		if (!result.ok) {
			// A missing version is not a screen failure: the release list is still worth reading. The
			// status line says the version is unknown rather than marking every release as newer.
			runningVersion = '';
			error = result.error.message;
			return;
		}

		runningVersion = result.data.version;
	}

	const releases = $derived(sortChangelog(entries));
	const newerCount = $derived(countNewer(releases, runningVersion));
</script>

<section class="flex max-w-3xl flex-col gap-6">
	<div class="flex flex-wrap items-start gap-4">
		<div class="flex min-w-0 flex-col gap-1">
			<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>
			<p class="text-sm text-[var(--color-text-muted)]">{copy.subtitle}</p>
		</div>

		<!-- The running version is the reason the screen exists, so it is the focal point rather than a
		     line of chrome. -->
		<div
			class="ms-auto flex flex-col items-end gap-0.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2"
		>
			<span
				class="text-[11px] font-semibold tracking-[0.06em] uppercase text-[var(--color-text-muted)]"
				>{copy.running.label}</span
			>
			<span class="font-mono text-sm text-[var(--color-text)]">
				{runningVersion || 'unknown'}
			</span>
		</div>
	</div>

	{#if error}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			{copy.running.unknown}
			<span class="block">{error}</span>
		</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title={copy.loading} />
	{:else if releases.length === 0}
		<StateMessage kind="empty" title={copy.empty.title} description={copy.empty.description} />
	{:else}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			{#if runningVersion}
				{newerCount > 0 ? copy.status.behind(newerCount) : copy.status.upToDate}
			{:else}
				{copy.status.unknown}
			{/if}
		</p>

		<ol class="flex flex-col gap-3">
			{#each releases as release (release.version)}
				{@const marker = releaseMarker(release.version, runningVersion)}
				<li
					class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4"
				>
					<div class="flex flex-wrap items-center gap-3">
						<span class="font-mono text-sm font-medium">{release.version}</span>
						<span class="text-xs text-[var(--color-text-muted)]">
							{formatTimestamp(release.released_at)}
						</span>
						<!-- A real status, not decoration: it says whether this build already has the release. -->
						{#if copy.marker[marker]}
							<span
								class="ms-auto rounded-[var(--radius-sm)] border px-1.5 py-0.5 text-[11px] {marker ===
								'running'
									? 'border-[var(--color-accent)] text-[var(--color-accent)]'
									: 'border-[var(--color-border)] text-[var(--color-text-muted)]'}"
							>
								{copy.marker[marker]}
							</span>
						{/if}
					</div>

					<div class="flex flex-col gap-1.5">
						<span
							class="text-[11px] font-semibold tracking-[0.06em] uppercase text-[var(--color-text-muted)]"
							>{copy.category[release.category]}</span
						>
						<ul class="flex flex-col gap-1">
							{#each release.items as item (item)}
								<li class="flex gap-2 text-sm">
									<!-- The identity motif at list scale: a marker on the leading edge, paired with
									     the text so meaning is never carried by the mark alone. -->
									<span
										aria-hidden="true"
										class="mt-1.5 size-1.5 shrink-0 rounded-[var(--radius-full)] bg-[var(--color-border)]"
									></span>
									<span class="min-w-0">{item}</span>
								</li>
							{/each}
						</ul>
					</div>

					<p class="text-xs text-[var(--color-text-muted)]">
						{copy.entriesCounted(release.items.length)}
					</p>
				</li>
			{/each}
		</ol>
	{/if}
</section>
