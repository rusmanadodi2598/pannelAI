<script lang="ts">
	// The send form (docs/SPEC-UI/001-SPEC-UI.md §6.15).
	//
	// Two fields and one control. The sentence beside the control is a requirement rather than decoration:
	// §6.15 rule 4 says the screen states the cost before the operator sends one, so it is here, next to
	// the button, and not something the answer area reports afterwards.
	//
	// The draft is this component's own state and the page is told only what to send, which keeps the
	// validation and the disable rule in one place. The form validates through the panel's own request
	// schema, so the body that leaves this screen is the body the panel's route accepts; the disabled
	// attribute is the same rule one step earlier, for the two cases it can see.
	import FormIssues from '$lib/components/FormIssues.svelte';
	import {
		schemaPlaygroundRequest,
		type PlaygroundModel,
		type PlaygroundRequest
	} from '$lib/schemas/playground';
	import { PLAYGROUND_COPY as copy } from '$lib/strings/playground';

	type Props = {
		/** The models the gateway said this key routes. The page renders this form only when there is one. */
		models: PlaygroundModel[];
		/** A send is in flight, so the control offers to stop it rather than to start another. */
		sending: boolean;
		/** Runs the send. Resolves when the stream has ended, however it ended. */
		onsend: (request: PlaygroundRequest) => Promise<void>;
		onstop: () => void;
	};

	let { models, sending, onsend, onstop }: Props = $props();

	let model = $state('');
	let message = $state('');
	let issues = $state<string[]>([]);

	// The first row becomes the draft's model once the list is there, rather than at construction: reading
	// the prop in the state initializer would capture the first value only and leave the select blank if the
	// list arrived later. The guard makes the write run once.
	$effect(() => {
		if (model === '' && models.length > 0) model = models[0].id;
	});

	// The disable rule covers the two cases an attribute can see: no model, and an empty box. A message of
	// whitespace is left enabled on purpose, because the schema is the authority on what a message is and
	// pressing Send there answers with the schema's own sentence rather than with a dead control (R-26).
	const canSend = $derived(model !== '' && message !== '');

	async function submit(): Promise<void> {
		const parsed = schemaPlaygroundRequest.safeParse({ model, message });

		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			return;
		}

		issues = [];
		await onsend(parsed.data);
	}
</script>

<form
	class="flex flex-col gap-3"
	onsubmit={(event) => {
		event.preventDefault();
		void submit();
	}}
>
	<div class="flex flex-col gap-1 text-sm">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">{copy.composer.modelLabel}</span>
			<select
				bind:value={model}
				class="min-h-11 w-full max-w-md rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			>
				{#each models as row (row.id)}
					<option value={row.id}>{row.owned_by ? `${row.id} (${row.owned_by})` : row.id}</option>
				{/each}
			</select>
		</label>
		<!-- Outside the label, so the control's accessible name stays the word above it rather than the hint. -->
		<span class="text-xs text-[var(--color-text-muted)]">{copy.composer.modelNote}</span>
	</div>

	<label class="flex flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">{copy.composer.messageLabel}</span>
		<textarea
			bind:value={message}
			rows="3"
			class="w-full resize-y rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-2"
			placeholder={copy.composer.messagePlaceholder}></textarea>
	</label>

	<div class="flex flex-wrap items-center gap-x-4 gap-y-2">
		{#if sending}
			<button
				type="button"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4"
				onclick={onstop}>{copy.composer.stop}</button
			>
		{:else}
			<button
				type="submit"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
				disabled={!canSend}>{copy.composer.send}</button
			>
		{/if}

		<p class="text-sm text-[var(--color-text-muted)]">{copy.composer.cost}</p>
	</div>

	<FormIssues {issues} />
</form>
