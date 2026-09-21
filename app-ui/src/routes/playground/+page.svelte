<script lang="ts">
	// Playground Chat (docs/SPEC-UI/001-SPEC-UI.md §6.15).
	//
	// The screen sends one message through the data plane and shows what came back. Three states are the
	// point rather than the decoration:
	//
	//   The panel has no key. The route answers 503 with a code of its own and a sentence that names the
	//   variable, and this screen renders that sentence rather than a sentence of its own, because the
	//   browser is not allowed to know the variable's name (see `src/lib/strings/playground.ts`). No
	//   control is rendered in this state: a send control that cannot send is a dead control (R-26).
	//
	//   The models list. It is what the gateway says this key can route, so an empty list is a fact about
	//   the key, and the screen says so instead of offering a picker with nothing in it.
	//
	//   The answer. The stream arrives frame by frame, so the text grows and the facts are filled in as
	//   the wire states them. The cost is stated by the composer before any of this happens.
	import { onMount } from 'svelte';
	import PlaygroundAnswer from '$lib/components/PlaygroundAnswer.svelte';
	import PlaygroundComposer from '$lib/components/PlaygroundComposer.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import {
		PLAYGROUND_MODELS_PATH,
		fetchPlaygroundModels,
		streamPlaygroundChat,
		type PlaygroundEndReason
	} from '$lib/api/playground';
	import { EMPTY_ANSWER, type AnswerState } from '$lib/schemas/playground-stream';
	import type {
		PlaygroundFailure,
		PlaygroundModel,
		PlaygroundRequest
	} from '$lib/schemas/playground';
	import { PLAYGROUND_COPY as copy, playgroundFailureDetail } from '$lib/strings/playground';

	let models = $state<PlaygroundModel[] | null>(null);
	let loading = $state(true);
	let loadFailure = $state<PlaygroundFailure | null>(null);

	let sending = $state(false);
	let answer = $state<AnswerState>(EMPTY_ANSWER);
	let raw = $state('');
	let status = $state<number | null>(null);
	let endReason = $state<PlaygroundEndReason | null>(null);
	let sendFailure = $state<PlaygroundFailure | null>(null);
	let controller: AbortController | null = null;

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		loadFailure = null;

		const result = await fetchPlaygroundModels();
		loading = false;

		if (!result.ok) {
			loadFailure = result.failure;
			return;
		}

		models = result.data.data;
	}

	async function send(request: PlaygroundRequest): Promise<void> {
		// One send at a time. The control that starts a send becomes the control that stops it, so a
		// second send cannot be started while one is running; this guard is the same rule for a submit
		// that arrives another way.
		if (sending) return;

		answer = EMPTY_ANSWER;
		raw = '';
		status = null;
		endReason = null;
		sendFailure = null;
		sending = true;

		const abort = new AbortController();
		controller = abort;

		await streamPlaygroundChat(
			request,
			{
				onStart: (value) => {
					status = value;
				},
				onUpdate: (next, frames) => {
					answer = next;
					raw = frames;
				},
				onEnd: (reason) => {
					endReason = reason;
				},
				onFailure: (failure) => {
					sendFailure = failure;
				}
			},
			abort.signal
		);

		// Cleared so a later Stop cannot abort a stream that already ended.
		controller = null;
		sending = false;
	}

	function stop(): void {
		controller?.abort();
	}

	// The answer block stays on screen for a stream that broke part way through, because the text that
	// arrived is still worth reading and the failure block above says what happened to it.
	const hasAnswer = $derived(sending || endReason !== null || answer.text !== '');
</script>

<section class="flex max-w-4xl flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">{copy.title}</h1>
		<p class="text-sm text-[var(--color-text-muted)]">{copy.subtitle}</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title={copy.models.loading} />
	{:else if loadFailure?.error.code === 'PLAYGROUND_KEY_MISSING'}
		<!-- The description is the panel's own sentence, which is the one that names the variable. -->
		<StateMessage
			kind="empty"
			title={copy.models.unavailableTitle}
			description={`${loadFailure.error.message} ${copy.models.unavailableNote}`}
		/>
	{:else if loadFailure}
		<StateMessage
			kind="error"
			title={copy.models.errorTitle}
			description={playgroundFailureDetail(loadFailure)}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}
					>{copy.models.retry}</button
				>
			{/snippet}
		</StateMessage>
	{:else if models && models.length === 0}
		<StateMessage
			kind="empty"
			title={copy.models.emptyTitle}
			description={copy.models.emptyDescription}
		/>
	{:else if models}
		<!-- What was read, stated as facts about this screen rather than as a claim about the gateway. -->
		<div
			class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm text-[var(--color-text-muted)]"
		>
			<span>{copy.models.readFrom(PLAYGROUND_MODELS_PATH)}</span>
			<span>{copy.models.count(models.length)}</span>
		</div>

		<PlaygroundComposer {models} {sending} onsend={send} onstop={stop} />

		{#if sendFailure}
			<StateMessage
				kind="error"
				title={copy.failure.title}
				description={playgroundFailureDetail(sendFailure)}
			/>
		{/if}

		{#if hasAnswer}
			<PlaygroundAnswer {answer} {raw} {endReason} {status} {sending} />
		{/if}
	{/if}
</section>
