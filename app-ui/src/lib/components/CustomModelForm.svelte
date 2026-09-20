<script lang="ts">
	// Adds one custom model to this provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The provider is fixed by the screen, so the form states it rather than offering a field that could
	// name another one. Capabilities are free text because the API accepts any value and publishes no
	// vocabulary: a picker would have to invent one, and a value outside it would be unreachable.
	//
	// The schema is the only validator (SPEC-UI §7.1). Its refusals are the API's own rules, so a save the
	// panel blocks is a save the gateway would have rejected, and the messages say what to change.
	import FormIssues from '$lib/components/FormIssues.svelte';
	import { createCustomModel } from '$lib/api/models';
	import {
		CAPABILITY_MAX_ENTRIES,
		customModelBody,
		customModelDraftEmpty,
		schemaCustomModelForm,
		type CustomModel
	} from '$lib/schemas/custom-model';

	let { providerId, onadded }: { providerId: string; onadded: (row: CustomModel) => void } =
		$props();

	let draft = $state(customModelDraftEmpty());
	let issues = $state<string[]>([]);
	let error = $state<string | null>(null);
	let adding = $state(false);

	async function submit(): Promise<void> {
		const parsed = schemaCustomModelForm.safeParse(draft);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			error = null;
			return;
		}

		issues = [];
		adding = true;
		const result = await createCustomModel(customModelBody(providerId, parsed.data));
		adding = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		draft = customModelDraftEmpty();
		onadded(result.data);
	}
</script>

<form
	class="flex flex-col gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<span class="font-medium">Add a custom model</span>
	<p class="text-sm text-[var(--color-text-muted)]">
		A custom model is one the registry does not carry. It is routable like any other model and the
		catalog lists it as custom.
	</p>

	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Model id</span>
			<input
				bind:value={draft.model_id}
				class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder="gpt-4o-mini"
			/>
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Display name</span>
			<input
				bind:value={draft.display_name}
				class="min-h-11 w-56 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder="GPT-4o mini"
			/>
		</label>
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Capabilities (optional)</span>
			<input
				bind:value={draft.capabilities}
				class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder="vision, tools"
			/>
		</label>
		<button
			type="submit"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
			disabled={adding}
		>
			{adding ? 'Adding' : 'Add the model'}
		</button>
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">
		No spaces in a model id. Capabilities are free text, comma separated, up to
		{CAPABILITY_MAX_ENTRIES}: the catalog filter offers vision and tools, and a value outside that
		list is still stored and shown.
	</p>

	<FormIssues {issues} />

	{#if error}
		<p class="text-sm text-[var(--color-danger)]" role="alert">The model was not added. {error}</p>
	{/if}
</form>
