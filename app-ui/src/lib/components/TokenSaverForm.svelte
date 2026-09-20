<script lang="ts">
	// The token saver editor (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// Three sections in the order §6.7 fixes: RTK, Headroom, Ponytail. Each saves on its own, and every
	// save is a whole-document PUT, because that is the only write route the API has. The draft helpers
	// merge the edited group over the last document that was read, which is what keeps one save from
	// rewriting a group the operator did not touch. After a successful write the page re-reads the full
	// configuration and hands it back through `loaded`.
	//
	// `caveman` is DEPRECATED and has no section, no control, and no label here (§6.7, owner 2026-09-16).
	import { untrack } from 'svelte';
	import TokenSaverBypass from '$lib/components/TokenSaverBypass.svelte';
	import TokenSaverHeadroomFields from '$lib/components/TokenSaverHeadroomFields.svelte';
	import TokenSaverPonytailFields from '$lib/components/TokenSaverPonytailFields.svelte';
	import TokenSaverRtkFilters from '$lib/components/TokenSaverRtkFilters.svelte';
	import TokenSaverSectionShell from '$lib/components/TokenSaverSectionShell.svelte';
	import { replaceTokenSaver } from '$lib/api/token-saver';
	import {
		buildTokenSaverBody,
		TOKEN_SAVER_SECTION_SCHEMAS,
		type TokenSaver,
		type TokenSaverSection
	} from '$lib/schemas/token-saver';
	import {
		tokenSaverCopy,
		tokenSaverSaveDraft,
		tokenSaverSectionDirty
	} from '$lib/schemas/token-saver-form';

	type Props = {
		loaded: TokenSaver;
		/** Asks the page to re-read the whole configuration after a successful write. */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	let draft = $state<TokenSaver>(untrack(() => tokenSaverCopy(loaded)));
	let server = $state<TokenSaver>(untrack(() => tokenSaverCopy(loaded)));

	let saving = $state<TokenSaverSection | null>(null);
	let saved = $state<TokenSaverSection | null>(null);
	let error = $state<string | null>(null);

	// Follows the document the page last read, so a re-read after a write is what the form shows rather
	// than a local guess about what the API stored.
	$effect(() => {
		const next = tokenSaverCopy(loaded);
		draft = next;
		server = next;
	});

	const dirty = $derived({
		rtk: tokenSaverSectionDirty('rtk', server, draft),
		headroom: tokenSaverSectionDirty('headroom', server, draft),
		ponytail: tokenSaverSectionDirty('ponytail', server, draft)
	});

	async function saveSection(section: TokenSaverSection): Promise<void> {
		error = null;
		saved = null;

		// Only the group being saved is validated. A half-typed value in another group is not a reason to
		// refuse this one, and it is not written either: the merge takes it from the last read.
		const parsed = TOKEN_SAVER_SECTION_SCHEMAS[section].safeParse(draft[section]);
		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = section;
		const merged = tokenSaverSaveDraft(section, draft, server);
		const result = await replaceTokenSaver(buildTokenSaverBody(merged));
		saving = null;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		const next = tokenSaverCopy(result.data);
		draft = next;
		server = next;
		saved = section;
		await onrefresh();
	}

	// Discards every unsaved group, not only the one whose control was pressed: the button belongs to a
	// section but the draft is one object, and pretending otherwise would need a second draft to hold.
	function discard(): void {
		draft = tokenSaverCopy(server);
		error = null;
		saved = null;
	}
</script>

<div class="flex flex-col gap-5">
	{#if error}
		<p
			role="alert"
			class="rounded-[var(--radius-sm)] border border-[var(--color-danger)] px-3 py-2 text-sm"
		>
			{error}
		</p>
	{/if}

	<TokenSaverSectionShell
		title="RTK"
		description="The gateway's own compressor, applied to tool results before they are sent upstream."
		dirty={dirty.rtk}
		saving={saving === 'rtk'}
		saved={saved === 'rtk'}
		saveLabel="Save RTK"
		onsave={() => void saveSection('rtk')}
		ondiscard={discard}
	>
		<label class="flex items-start gap-3 text-sm">
			<input type="checkbox" class="mt-1 size-4" bind:checked={draft.rtk.enabled} />
			<span>
				Compress tool results with the native filters
				<span class="block text-xs text-[var(--color-text-muted)]">
					Errors are preserved, and a blob a filter would grow is left as it was.
				</span>
			</span>
		</label>

		<TokenSaverRtkFilters
			filters={draft.rtk.filters}
			onchange={(filters) => (draft.rtk.filters = filters)}
		/>
	</TokenSaverSectionShell>

	<TokenSaverSectionShell
		title="Headroom"
		description="An external compression service. The gateway calls it and sends what comes back."
		dirty={dirty.headroom}
		saving={saving === 'headroom'}
		saved={saved === 'headroom'}
		saveLabel="Save Headroom"
		onsave={() => void saveSection('headroom')}
		ondiscard={discard}
	>
		<TokenSaverHeadroomFields bind:value={draft.headroom} />
	</TokenSaverSectionShell>

	<TokenSaverSectionShell
		title="Ponytail"
		description="A bias the gateway adds to the request, so the model answers in the shorter shape."
		dirty={dirty.ponytail}
		saving={saving === 'ponytail'}
		saved={saved === 'ponytail'}
		saveLabel="Save Ponytail"
		onsave={() => void saveSection('ponytail')}
		ondiscard={discard}
	>
		<TokenSaverPonytailFields bind:value={draft.ponytail} />
	</TokenSaverSectionShell>

	<TokenSaverBypass />
</div>
