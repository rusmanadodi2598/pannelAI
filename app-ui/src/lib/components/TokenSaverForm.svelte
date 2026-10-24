<script lang="ts">
	// The token saver editor (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// Three sections in the order §6.7 fixes: RTK, Headroom, Ponytail. Each saves on its own, and every
	// save is a whole-document PUT, because that is the only write route the API has. The draft helpers
	// merge the edited group over the last document that was read, which is what keeps one save from
	// rewriting a group the operator did not touch. After a successful write the page re-reads the full
	// configuration and hands it back through `loaded`.
	//
	// `caveman` is deprecated and has no section, no control, and no label here (§6.7).
	import { untrack } from 'svelte';
	import TokenSaverBypass from '$lib/components/TokenSaverBypass.svelte';
	import TokenSaverHeadroomFields from '$lib/components/TokenSaverHeadroomFields.svelte';
	import TokenSaverPonytailFields from '$lib/components/TokenSaverPonytailFields.svelte';
	import TokenSaverRtkFilters from '$lib/components/TokenSaverRtkFilters.svelte';
	import TokenSaverSectionShell from '$lib/components/TokenSaverSectionShell.svelte';
	import { replaceTokenSaver } from '$lib/api/token-saver';
	import type { ApiError } from '$lib/api/errors';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		buildTokenSaverBody,
		TOKEN_SAVER_SECTION_SCHEMAS,
		type TokenSaver,
		type TokenSaverSection
	} from '$lib/schemas/token-saver';
	import {
		tokenSaverCopy,
		tokenSaverDiscardSection,
		tokenSaverMergeReload,
		tokenSaverSaveDraft,
		tokenSaverSaveResult,
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
	let error = $state<string | ApiError | null>(null);
	let errorSection = $state<TokenSaverSection | null>(null);

	// A re-read after a save is the document the form follows, but a group the operator is editing in
	// another section has not been saved and must survive that read.
	$effect(() => {
		const reloaded = loaded;
		untrack(() => {
			draft = tokenSaverMergeReload(draft, server, reloaded);
			server = tokenSaverCopy(reloaded);
		});
	});

	const dirty = $derived({
		rtk: tokenSaverSectionDirty('rtk', server, draft),
		headroom: tokenSaverSectionDirty('headroom', server, draft),
		ponytail: tokenSaverSectionDirty('ponytail', server, draft)
	});

	// §8.4.4: the guard takes any unsaved group, because leaving the screen loses all three at once.
	$effect(() => registerDirtyForm(() => Object.values(dirty).some(Boolean)));

	async function saveSection(section: TokenSaverSection): Promise<void> {
		if (saving !== null) return;

		error = null;
		errorSection = null;
		saved = null;

		// Only the group being saved is validated. A half-typed value in another group is not a reason to
		// refuse this one, and it is not written either: the merge takes it from the last read.
		const parsed = TOKEN_SAVER_SECTION_SCHEMAS[section].safeParse(draft[section]);
		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'Check these values.';
			errorSection = section;
			return;
		}

		const submitted = tokenSaverCopy(draft);
		const merged = tokenSaverSaveDraft(section, submitted, server);
		saving = section;

		try {
			const result = await replaceTokenSaver(buildTokenSaverBody(merged));

			if (!result.ok) {
				error = result.error;
				errorSection = section;
				return;
			}

			const outcome = tokenSaverSaveResult(section, draft, server, result.data, submitted);
			draft = outcome.draft;
			server = tokenSaverCopy(result.data);
			saved = outcome.saved ? section : null;
			// The save is not complete until the page has re-read it, and no second section may write in
			// that window.
			await onrefresh();
		} finally {
			saving = null;
		}
	}

	function discard(section: TokenSaverSection): void {
		if (saving !== null) return;

		draft = tokenSaverDiscardSection(section, draft, server);
		if (errorSection === section) {
			error = null;
			errorSection = null;
		}
		if (saved === section) saved = null;
	}
</script>

<div class="flex flex-col gap-5">
	<TokenSaverSectionShell
		title="RTK"
		description="Built-in compaction for tool output."
		dirty={dirty.rtk}
		saving={saving === 'rtk'}
		saved={saved === 'rtk'}
		disabled={saving !== null}
		error={errorSection === 'rtk' ? error : null}
		saveLabel="Save RTK"
		onsave={() => void saveSection('rtk')}
		ondiscard={() => discard('rtk')}
	>
		<label class="flex min-h-11 items-start gap-3 text-sm">
			<input type="checkbox" class="mt-1 size-4" bind:checked={draft.rtk.enabled} />
			<span>
				Compress tool results with the native filters
				<span class="block text-xs text-[var(--color-text-muted)]">
					Errors and growing output are left unchanged.
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
		description="A compression service called upstream."
		dirty={dirty.headroom}
		saving={saving === 'headroom'}
		saved={saved === 'headroom'}
		disabled={saving !== null}
		error={errorSection === 'headroom' ? error : null}
		saveLabel="Save Headroom"
		onsave={() => void saveSection('headroom')}
		ondiscard={() => discard('headroom')}
	>
		<TokenSaverHeadroomFields bind:value={draft.headroom} />
	</TokenSaverSectionShell>

	<TokenSaverSectionShell
		title="Ponytail"
		description="A bias added to the request for shorter answers."
		dirty={dirty.ponytail}
		saving={saving === 'ponytail'}
		saved={saved === 'ponytail'}
		disabled={saving !== null}
		error={errorSection === 'ponytail' ? error : null}
		saveLabel="Save Ponytail"
		onsave={() => void saveSection('ponytail')}
		ondiscard={() => discard('ponytail')}
	>
		<TokenSaverPonytailFields bind:value={draft.ponytail} />
	</TokenSaverSectionShell>

	<TokenSaverBypass />
</div>
