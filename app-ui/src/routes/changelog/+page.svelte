<script lang="ts">
	// Changelog (docs/SPEC-UI/001-SPEC-UI.md §6.16).
	//
	// The screen answers one question: what changed, and is this build behind? So the hierarchy is the
	// running version first, then the releases, newest first, each marked against that version.
	//
	// Two reads, and the screen says which one failed. The releases come from `GET /api/v1/changelog`
	// (SPEC-API §7.18), which the binary serves from the history it was built with; the running version
	// comes from `GET /api/v1/version` (§7.1), and it is what turns a list into an answer. A version that
	// could not be read does not hide the list: the releases are worth reading without it, and the status
	// line says the comparison is unavailable rather than marking every release as newer.
	import { onMount } from 'svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { CHANGELOG_PATH, fetchChangelog } from '$lib/api/changelog';
	import { fetchSystemInfo } from '$lib/api/system';
	import {
		canCompare,
		countNewer,
		releaseMarker,
		sortChangelog,
		type Changelog,
		type ReleaseMarker
	} from '$lib/schemas/changelog';
	import { CHANGELOG_COPY as copy } from '$lib/strings/changelog';

	let changelog = $state<Changelog | null>(null);
	let runningVersion = $state('');
	let versionError = $state<string | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		versionError = null;

		// Both reads at once: neither answer depends on the other, and asking for the second only after the
		// first landed would spend two round trips on two facts the screen can have in one.
		const [list, version] = await Promise.all([fetchChangelog(), fetchSystemInfo()]);
		loading = false;

		if (!list.ok) {
			changelog = null;
			error = list.error.message;
			return;
		}

		changelog = list.data;

		if (!version.ok) {
			runningVersion = '';
			versionError = version.error.message;
			return;
		}

		runningVersion = version.data.version;
	}

	const releases = $derived(changelog ? sortChangelog(changelog.data) : []);
	const newerCount = $derived(countNewer(releases, runningVersion));
	const comparable = $derived(canCompare(runningVersion));

	// The identity motif carries the marker and the chip beside it says the same thing in words, so the
	// state is never carried by the colour alone. `unknown` gets neither, because a mark with no sentence
	// next to it would be the one thing the motif may not do.
	const MARKER_TONES: Record<ReleaseMarker, string> = {
		running: 'bg-[var(--color-accent)]',
		newer: 'bg-[var(--color-warn)]',
		older: 'bg-[var(--color-border)]',
		unknown: ''
	};
</script>

<section class="flex max-w-3xl flex-col gap-6">
	<div class="flex flex-wrap items-start gap-4">
		<div class="flex min-w-0 flex-col gap-1">
			<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>
			<p class="text-sm text-[var(--color-text-muted)]">{copy.subtitle}</p>
		</div>

		<!-- The running version is the reason the screen exists, so it is the focal point rather than a
		     line of chrome. While the read is in flight it says so, rather than claiming the version is
		     unknown before the gateway has answered. -->
		<div
			class="ms-auto flex flex-col items-end gap-0.5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2"
		>
			<span
				class="text-[11px] font-semibold tracking-[0.06em] uppercase text-[var(--color-text-muted)]"
				>{copy.running.label}</span
			>
			<span class="font-mono text-sm text-[var(--color-text)]">
				{loading ? copy.running.reading : runningVersion || 'unknown'}
			</span>
		</div>
	</div>

	<RefreshControl onrefresh={load} />

	{#if loading}
		<StateMessage kind="loading" title={copy.loading} />
	{:else if error}
		<StateMessage kind="error" title={copy.error.title} description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}
					>{copy.error.retry}</button
				>
			{/snippet}
		</StateMessage>
	{:else if changelog && releases.length === 0}
		<StateMessage
			kind="empty"
			title={copy.empty.title}
			description={copy.empty.description(CHANGELOG_PATH)}
		/>
	{:else if changelog}
		<!-- What was read, stated as facts about the route rather than as a claim about the gateway. -->
		<div
			class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm text-[var(--color-text-muted)]"
		>
			<span>{copy.source.readFrom(CHANGELOG_PATH)}</span>
			<span>{copy.source.releases(releases.length)}</span>
		</div>

		{#if versionError}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				{copy.running.unreadable}
				<span class="block">{versionError}</span>
			</p>
		{:else if !comparable}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				{copy.running.notComparable(runningVersion)}
			</p>
		{:else}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				{newerCount > 0 ? copy.status.behind(newerCount) : copy.status.upToDate}
			</p>
		{/if}

		<ol class="flex flex-col gap-3">
			{#each releases as release (release.version)}
				{@const marker = releaseMarker(release.version, runningVersion)}
				{@const label = copy.marker[marker]}
				<li
					class="relative flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 ps-5"
				>
					{#if label}
						<span
							aria-hidden="true"
							class="absolute inset-y-4 start-0 w-[3px] rounded-[var(--radius-full)] {MARKER_TONES[
								marker
							]}"
						></span>
					{/if}

					<div class="flex flex-wrap items-center gap-3">
						<span class="font-mono text-sm font-medium">{release.version}</span>
						<!-- The date is the gateway's own calendar date, rendered as it was sent. A date has no
						     zone, and a zoned formatter would print a day the gateway never reported. -->
						<time class="text-xs text-[var(--color-text-muted)]" datetime={release.date}
							>{release.date}</time
						>
						{#if label}
							<span
								class="ms-auto rounded-[var(--radius-sm)] border px-1.5 py-0.5 text-[11px] {marker ===
								'running'
									? 'border-[var(--color-accent)] text-[var(--color-accent)]'
									: 'border-[var(--color-border)] text-[var(--color-text-muted)]'}"
							>
								{label}
							</span>
						{/if}
					</div>

					<div class="flex flex-col gap-1">
						<h2 class="text-sm font-medium">{release.title}</h2>
						<p class="text-sm text-[var(--color-text-muted)]">{release.notes}</p>
					</div>
				</li>
			{/each}
		</ol>
	{/if}
</section>
