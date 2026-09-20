<script lang="ts">
	// The add-or-change row for the alias set (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// One form serves both actions. A name that is already in the set changes what that alias targets,
	// because `alias` is the table's primary key: a second row with the same name is a database error whose
	// answer names the constraint rather than the alias, so the merge in `withAlias` is both the only shape
	// that can succeed and the reason the section needs no separate edit control.
	//
	// The target field suggests catalog ids and combo names. They are suggestions, not a constraint: the API
	// resolves the target, and a stored target may have left both lists since it was written. A read that
	// failed therefore leaves this field usable, and `note` says so rather than leaving the operator to
	// wonder why the list is empty.
	import FormIssues from '$lib/components/FormIssues.svelte';
	import { schemaAliasEntry, type ModelAliasEntry } from '$lib/schemas/model-alias';

	type Props = {
		/** Catalog ids and combo names, for the target field's suggestions. */
		suggestions: string[];
		/** What to say about the suggestion list, or null when there is nothing to say. */
		note?: string | null;
		/** A write is in flight, so this form waits rather than merging over the set it is replacing. */
		busy: boolean;
		/** Runs the write. The draft clears only when it landed. */
		onadd: (entry: ModelAliasEntry) => Promise<boolean>;
	};

	let { suggestions, note = null, busy, onadd }: Props = $props();

	let draft = $state<ModelAliasEntry>({ alias: '', target: '' });
	let issues = $state<string[]>([]);

	async function submit(): Promise<void> {
		const parsed = schemaAliasEntry.safeParse(draft);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			return;
		}

		issues = [];
		if (await onadd(parsed.data)) {
			draft = { alias: '', target: '' };
		}
	}
</script>

<form
	class="flex flex-col gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<span class="font-medium">Add an alias</span>

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Alias</span>
			<input
				bind:value={draft.alias}
				class="min-h-11 w-56 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder="fast"
			/>
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Target</span>
			<input
				bind:value={draft.target}
				list="alias-targets"
				class="min-h-11 w-72 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder="openai/gpt-4o-mini"
			/>
			<datalist id="alias-targets">
				{#each suggestions as suggestion (suggestion)}
					<option value={suggestion}></option>
				{/each}
			</datalist>
		</label>
		<button
			type="submit"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
			disabled={busy}
		>
			{busy ? 'Saving' : 'Add the alias'}
		</button>
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">
		A target is a provider/model reference or a combo name. An alias that is already in the table is
		changed here rather than added twice.
	</p>

	{#if note}
		<p class="text-sm text-[var(--color-text-muted)]">{note}</p>
	{/if}

	<FormIssues {issues} />
</form>
