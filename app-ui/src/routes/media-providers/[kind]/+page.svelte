<script lang="ts">
	// Media providers by kind (docs/SPEC-UI/001-SPEC-UI.md §6.8).
	//
	// One screen at six addresses. The page's job is the kind, the load, and the three states; each
	// provider is a card that owns its own draft and save.
	//
	// The kind is checked before anything is requested, because the API refuses an unknown kind with a
	// 400 and a page that asked anyway would report a validation error for what is really a bad URL. An
	// unknown segment gets its own state naming the six, so the operator can pick one instead of guessing.
	//
	// A save does not refetch the list: the PATCH answers with the resolved block for the kind it wrote,
	// which is exactly the row the card rendered, so the page replaces that row in place.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import MediaProviderCard from '$lib/components/MediaProviderCard.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listMediaProviders } from '$lib/api/media-providers';
	import {
		MEDIA_KINDS,
		isMediaKind,
		mediaKindLabel,
		type MediaKind,
		type MediaKindBlock,
		type MediaProvider
	} from '$lib/schemas/media-provider';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const slug = $derived(params.kind);
	const kind = $derived(isMediaKind(slug) ? slug : null);

	let providers = $state<MediaProvider[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Reloads when the route's kind changes, so a hand-edited URL never leaves one kind's rows under
	// another kind's heading.
	$effect(() => {
		const current = kind;
		untrack(() => {
			if (current === null) {
				loading = false;
				providers = [];
				error = null;
				return;
			}
			void load(current);
		});
	});

	async function load(current: MediaKind): Promise<void> {
		loading = true;
		const result = await listMediaProviders(current);
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			providers = [];
			return;
		}

		error = null;
		providers = result.data.data;
	}

	function applyResolved(providerId: string, block: MediaKindBlock): void {
		providers = providers.map((row) =>
			row.provider_id === providerId ? { ...row, ...block } : row
		);
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">
			{kind === null ? 'Media providers' : mediaKindLabel(kind)}
		</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			The registry's providers for this kind, with the address each one is dialed at. An override
			here changes what the gateway uses for this kind on the next request.
		</p>
	</div>

	{#if kind === null}
		<StateMessage
			kind="error"
			title={`There is no ${slug} media kind`}
			description="The panel has six media kinds, and the address names one of them."
		>
			{#snippet action()}
				<ul class="flex flex-wrap gap-x-4 gap-y-1 text-sm">
					{#each MEDIA_KINDS as candidate (candidate)}
						<li>
							<a href={resolve('/media-providers/[kind]', { kind: candidate })} class="underline">
								{mediaKindLabel(candidate)}
							</a>
						</li>
					{/each}
				</ul>
			{/snippet}
		</StateMessage>
	{:else if loading}
		<StateMessage kind="loading" title="Loading the providers for this kind" />
	{:else if error}
		<StateMessage kind="error" title="These providers could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load(kind)}>Try again</button>
				<a href={resolve('/providers')} class="ms-3 underline">Back to the registry</a>
			{/snippet}
		</StateMessage>
	{:else if providers.length === 0}
		<StateMessage
			kind="empty"
			title="No provider configured for this kind."
			description="The registry declares no provider that offers this kind, so there is nothing to configure here yet."
		>
			{#snippet action()}
				<a href={resolve('/providers')} class="underline">Open Providers</a>
			{/snippet}
		</StateMessage>
	{:else}
		<div class="flex flex-col gap-4">
			{#each providers as provider (provider.provider_id)}
				<MediaProviderCard {provider} {kind} onresolved={applyResolved} />
			{/each}
		</div>
	{/if}
</section>
