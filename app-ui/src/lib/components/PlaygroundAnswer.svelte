<script lang="ts">
	// The streamed answer (docs/SPEC-UI/001-SPEC-UI.md §6.15).
	//
	// §6.15 rule 4 asks the screen to show the resolved model and the upstream status so a failure is
	// diagnosable rather than mysterious, and the facts row is that requirement. Every value in it is
	// reported rather than assumed: a fact the wire did not state reads as not stated, because a blank
	// would look like a fact and a value this screen invented would be worse than either.
	//
	// The leading marker is DESIGN.md §6's identity motif carrying the stream's state, and it never
	// appears alone: the sentence beside it says the same thing in words.
	import type { PlaygroundEndReason } from '$lib/api/playground';
	import type { AnswerState } from '$lib/schemas/playground-stream';
	import { PLAYGROUND_COPY as copy, usageSentence } from '$lib/strings/playground';

	type Props = {
		answer: AnswerState;
		/** The frames exactly as they arrived, for the disclosure at the bottom. */
		raw: string;
		/** Null while the stream is running, and after one that broke rather than ended. */
		endReason: PlaygroundEndReason | null;
		/** The status the panel's route answered with, or null when no answer was read. */
		status: number | null;
		sending: boolean;
	};

	let { answer, raw, endReason, status, sending }: Props = $props();

	const marker = $derived(
		sending
			? 'bg-[var(--color-accent)]'
			: endReason === 'done'
				? 'bg-[var(--color-ok)]'
				: 'bg-[var(--color-warn)]'
	);

	const stateSentence = $derived(
		sending
			? answer.text === ''
				? copy.answer.waiting
				: copy.answer.streaming
			: endReason === null
				? copy.answer.interrupted
				: copy.answer.end[endReason]
	);

	const facts = $derived([
		{ label: copy.answer.facts.model, value: answer.model ?? copy.answer.facts.notStated },
		{ label: copy.answer.facts.finish, value: answer.finishReason ?? copy.answer.facts.notStated },
		{ label: copy.answer.facts.tokens, value: usageSentence(answer.usage) },
		{
			label: copy.answer.facts.status,
			value: status === null ? copy.answer.facts.notStated : String(status)
		}
	]);
</script>

<section
	class="relative flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-4 ps-5"
>
	<span
		class="absolute inset-y-4 start-0 w-[3px] rounded-[var(--radius-full)] {marker}"
		aria-hidden="true"
	></span>

	<h2 class="text-base font-medium">{copy.answer.heading}</h2>

	<p class="text-sm text-[var(--color-text-muted)]" role="status">{stateSentence}</p>

	{#if answer.text !== ''}
		<div
			class="rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm whitespace-pre-wrap"
		>
			{answer.text}
		</div>
	{:else if !sending}
		<p class="text-sm text-[var(--color-text-muted)]">{copy.answer.empty}</p>
	{/if}

	<dl class="flex flex-wrap gap-x-6 gap-y-2 border-t border-[var(--color-border)] pt-3 text-sm">
		{#each facts as fact (fact.label)}
			<div class="flex flex-col">
				<dt class="text-xs text-[var(--color-text-muted)]">{fact.label}</dt>
				<dd class="font-mono text-xs break-all">{fact.value}</dd>
			</div>
		{/each}
	</dl>

	{#if raw !== ''}
		<details class="border-t border-[var(--color-border)] pt-3">
			<summary class="cursor-pointer text-sm">{copy.answer.raw.summary}</summary>
			<p class="pt-2 text-xs text-[var(--color-text-muted)]">{copy.answer.raw.intro}</p>
			<pre
				class="mt-2 overflow-x-auto rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap"><code
					>{raw}</code
				></pre>
		</details>
	{/if}
</section>
