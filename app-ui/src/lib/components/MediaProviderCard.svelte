<script lang="ts">
	// One media provider card (docs/SPEC-UI/001-SPEC-UI.md §6.8).
	//
	// The card owns its own draft and its own save state, because the two fields it edits belong to one
	// provider and one kind. The page hands back the block the API resolved, so the card re-renders what
	// the gateway will dial rather than what it hoped it wrote.
	//
	// Two rules shape the controls. The base URL field is free text, so the panel validates it and blocks
	// the one save it can prove the server would refuse, quoting the server's own sentence. The model
	// field is a selector over the declared set, so an invalid model is impossible and needs no message.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { saveMediaOverride } from '$lib/api/media-providers';
	import {
		mediaBaseUrlRefusal,
		type MediaKind,
		type MediaKindBlock,
		type MediaProvider
	} from '$lib/schemas/media-provider';
	import {
		hasDeclaredModels,
		mediaBaseUrlHelp,
		mediaDraftFrom,
		mediaModelHelp,
		mediaModelOptions,
		mediaModelSelection,
		mediaOverrideBody,
		schemaMediaOverrideForm,
		undeclaredModel,
		type MediaOverrideDraft
	} from '$lib/schemas/media-provider-form';

	type Props = {
		provider: MediaProvider;
		kind: MediaKind;
		/** Handed the block the API resolved, so the page can replace the row it rendered. */
		onresolved: (providerId: string, block: MediaKindBlock) => void;
	};

	let { provider, kind, onresolved }: Props = $props();

	// `untrack` on the initialiser only: the row is read once to seed the draft, and the effect below is
	// what keeps the two in step. Without it the compiler warns that the reference captures the first
	// value, which is exactly what is wanted here and would be a defect anywhere else.
	let draft = $state<MediaOverrideDraft>(untrack(() => mediaDraftFrom(provider)));
	let saving = $state(false);
	/** The refusal the panel proved the server would answer with. */
	let refusal = $state<string | null>(null);
	/** The server's own refusal, which the panel could not predict. */
	let failure = $state<string | null>(null);
	let saved = $state<string | null>(null);

	// The card follows the row it renders. A save hands back a new row, and a card that kept its own
	// copy would show a stale base URL beside a fresh one.
	$effect(() => {
		const next = provider;
		untrack(() => {
			draft = mediaDraftFrom(next);
		});
	});

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
	const hintClass = 'text-xs text-[var(--color-text-muted)]';

	const modelOptions = $derived(mediaModelOptions(provider));
	const modelSelection = $derived(mediaModelSelection(provider));
	const undeclared = $derived(undeclaredModel(provider));

	async function save(): Promise<void> {
		refusal = null;
		failure = null;
		saved = null;

		const parsed = schemaMediaOverrideForm.safeParse(draft);
		if (!parsed.success) {
			failure = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		const blocked = mediaBaseUrlRefusal({
			providerId: provider.provider_id,
			block: provider,
			baseUrl: parsed.data.baseUrl
		});
		if (blocked) {
			refusal = blocked;
			return;
		}

		saving = true;
		const result = await saveMediaOverride(
			provider.provider_id,
			mediaOverrideBody(kind, parsed.data)
		);
		saving = false;

		if (!result.ok) {
			failure = result.error.message;
			return;
		}

		onresolved(provider.provider_id, result.data);
		saved = 'Saved. This is the address the gateway will dial for this kind.';
	}
</script>

<article
	class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-4 py-5"
	aria-labelledby={`media-${provider.provider_id}`}
>
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<div class="flex flex-col">
			<h2 id={`media-${provider.provider_id}`} class="text-base font-medium">
				<a
					href={resolve('/providers/[provider_id]', { provider_id: provider.provider_id })}
					class="underline"
				>
					{provider.provider_name}
				</a>
			</h2>
			<p class="text-sm text-[var(--color-text-muted)]">{provider.provider_id}</p>
		</div>
		<!-- The count is scoped to this provider and this kind, so it is stated rather than linked: the
		     endpoint screen filters by provider alone, and a link there would promise a narrower list. -->
		<p class="text-sm text-[var(--color-text-muted)]">
			{provider.endpoint_count}
			{provider.endpoint_count === 1 ? 'endpoint' : 'endpoints'}
			{provider.endpoint_count === 0 ? ' configured for this kind yet' : ' for this kind'}
		</p>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for={`media-base-url-${provider.provider_id}`}>Base URL</label>
		<input
			id={`media-base-url-${provider.provider_id}`}
			type="text"
			bind:value={draft.baseUrl}
			aria-describedby={`media-base-url-help-${provider.provider_id}`}
			class={fieldClass}
		/>
		<span id={`media-base-url-help-${provider.provider_id}`} class={hintClass}>
			{mediaBaseUrlHelp(provider)}
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		{#if hasDeclaredModels(provider)}
			<label for={`media-model-${provider.provider_id}`}>Default model</label>
			<select
				id={`media-model-${provider.provider_id}`}
				value={modelSelection}
				onchange={(event) => (draft.defaultModel = event.currentTarget.value)}
				aria-describedby={`media-model-help-${provider.provider_id}`}
				class={fieldClass}
			>
				{#each modelOptions as option (option.value)}
					<option value={option.value}>{option.label}</option>
				{/each}
			</select>
		{:else}
			<span>Default model</span>
		{/if}
		<span id={`media-model-help-${provider.provider_id}`} class={hintClass}>
			{mediaModelHelp(provider)}
		</span>
	</div>

	{#if undeclared}
		<p class="text-xs text-[var(--color-text-muted)]">
			Currently set to {undeclared}, which this service no longer declares. Choosing the registry
			default clears it.
		</p>
	{/if}

	{#if refusal}
		<p role="alert" class="text-sm text-[var(--color-danger)]">{refusal}</p>
	{:else if failure}
		<StateMessage kind="error" title="This change was not saved" description={failure} />
	{:else if saved}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{saved}</p>
	{/if}

	<div>
		<button
			type="button"
			onclick={() => void save()}
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
		>
			{saving ? 'Saving' : 'Save'}
		</button>
	</div>
</article>
