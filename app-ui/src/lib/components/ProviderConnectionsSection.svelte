<script lang="ts">
	// The Connections section of the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section is the reference's own block (`providers/[id]/page.js:1508-1741`): a heading, the
	// rotation switch, the action that adds a connection, and the list. It is its own component because
	// both kinds of provider screen render it and the two put it in different places: the registry
	// screen leads with its facts and its model sections, while a custom node leads with this, because a
	// node with no connection has nothing to route with yet.
	//
	// The add action belongs to the page rather than here: a key added from the node's own details card
	// opens the same dialog, so the page owns that state and hands this section the button and the notice
	// that reports the last add.
	//
	// The add action is split by auth type. A provider that takes a key gets the dialog the page owns,
	// because a key is the credential and the reference's own Add API Key button sits here. A provider that
	// takes none gets this section's inline endpoint form instead: since 2026-09-24 the Endpoint & Key page
	// carries only gateway keys, so the form that used to live on its second tab renders here, next to the
	// list it fills.
	//
	// The rotation switch is its own component, because it is a self-contained control with its own read,
	// write, and status lines; this section only decides where it sits.
	import CreateEndpointForm from '$lib/components/CreateEndpointForm.svelte';
	import ProviderEndpoints from '$lib/components/ProviderEndpoints.svelte';
	import ProviderRotationSwitch from '$lib/components/ProviderRotationSwitch.svelte';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import { REQUIRES_KEY_AUTH_TYPES } from '$lib/schemas/endpoint-write';
	import type { ProviderDetail } from '$lib/schemas/provider';

	let {
		provider,
		token = 0,
		notice = null,
		onaddkey
	}: {
		provider: ProviderDetail;
		/** Bumped by the page when a key was added, so the table below re-reads. */
		token?: number;
		/** The last add's outcome, reported by the page that owns the dialog. */
		notice?: string | null;
		onaddkey: () => void;
	} = $props();

	const AddKeyIcon = CONTROL_ICONS.addKey.icon;
	const CheckIcon = ROW_ACTION_ICONS.save.icon;
	const AddIcon = CONTROL_ICONS.add.icon;
	const CancelIcon = CONTROL_ICONS.clear.icon;

	// A key is only meaningful where the provider's own auth type takes one. For an OAuth provider the
	// dialog would ask for a credential the provider does not use, and the path that fits is the endpoint
	// form, which carries the auth type with it.
	const offersKeys = $derived(REQUIRES_KEY_AUTH_TYPES.has(provider.auth_type));

	// The inline create form's own state: the add button toggles it, and a successful create bumps a local
	// revision so the list below re-reads without a page reload.
	let creating = $state(false);
	let createRevision = $state(0);
	let createMessage = $state<string | null>(null);
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-base font-medium">Connections</h2>
		<div class="flex flex-wrap items-center gap-3">
			<ProviderRotationSwitch providerId={provider.id} />
			{#if offersKeys}
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
					onclick={onaddkey}
				>
					<AddKeyIcon class="size-4" aria-hidden="true" />
					Add API Key
				</button>
			{:else}
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
					onclick={() => (creating = !creating)}
				>
					{#if creating}
						<CancelIcon class="size-4" aria-hidden="true" />
						Hide the form
					{:else}
						<AddIcon class="size-4" aria-hidden="true" />
						Add a connection
					{/if}
				</button>
			{/if}
		</div>
	</div>
	{#if creating}
		<CreateEndpointForm
			providerId={provider.id}
			oncreated={() => {
				creating = false;
				createMessage = 'Connection added.';
				createRevision += 1;
			}}
		/>
	{/if}
	<!-- The page hands this section the last add's outcome, which is always a success (the dialog only
	     reports one), so the Check glyph marks a real state rather than decorating the line. -->
	{#if notice}
		<p class="inline-flex items-center gap-1.5 text-sm" role="status">
			<CheckIcon class="size-4" aria-hidden="true" />
			{notice}
		</p>
	{/if}
	{#if createMessage}
		<p class="text-sm" role="status">{createMessage}</p>
	{/if}
	<ProviderEndpoints providerId={provider.id} token={token + createRevision} />
</div>
