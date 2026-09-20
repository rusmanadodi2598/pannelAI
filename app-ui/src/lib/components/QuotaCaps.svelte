<script lang="ts">
	// The budget-cap editor for one endpoint (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
	//
	// A cap belongs to an endpoint, so this section is a picker plus one form rather than a column on the
	// window table: the collection route carries no cap, and reading one per endpoint to fill a column
	// would be an N+1 (SPEC-API §7.12). Choosing an endpoint reads its cap; saving writes the whole cap
	// set and then reads it back, so what is on screen is what the gateway kept rather than what was sent.
	//
	// The section is not gated on the window table. A cap is legal before the first routed request, which
	// is the state an operator setting a budget is most likely to be in, and a screen that hid this form
	// until traffic existed would hide it exactly then.
	//
	// A blank field is a value, not an omission: the route replaces the whole cap set, so an empty field
	// clears that cap. The form says so, because an operator who blanked a field to mean "leave it alone"
	// would otherwise clear a budget without being told.
	import FormIssues from '$lib/components/FormIssues.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { getQuotaEndpoint, replaceQuotaCap } from '$lib/api/usage';
	import {
		QUOTA_CAP_WARNING,
		buildQuotaCapBody,
		quotaCapForm,
		quotaCapOptions,
		quotaCapSummary,
		schemaQuotaCapForm,
		type QuotaCap
	} from '$lib/schemas/quota-cap';
	import { formatTimestamp } from '$lib/utils/time';
	import { untrack } from 'svelte';

	type Props = {
		/** The endpoint labels the screen read, keyed by id. Best effort, so it may be empty. */
		labels: Map<string, string>;
		/** Every endpoint the window table names, which can reach past the label list's first page. */
		windowEndpointIds: string[];
		/** Whether the screen's endpoint-list read failed, which is why the picker can be empty. */
		labelsUnread?: boolean;
	};

	let { labels, windowEndpointIds, labelsUnread = false }: Props = $props();

	const options = $derived(quotaCapOptions(labels, windowEndpointIds));

	let selected = $state('');
	// The last cap the gateway reported, with `undefined` meaning it has not answered yet. That is a state
	// of its own: before an answer, "no cap is stored" would be a claim the panel cannot make, and a field
	// the operator could type into would be overwritten by the answer when it landed.
	let read = $state<QuotaCap | null | undefined>(undefined);
	let draft = $state({ cost: '', tokens: '' });
	let loading = $state(false);
	let saving = $state(false);
	let saved = $state(false);
	let issues = $state<string[]>([]);
	let error = $state<string | null>(null);

	// One read per choice, and the draft is seeded from that read, so a save replaces the cap the gateway
	// holds rather than a value the operator could not see.
	$effect(() => {
		const id = selected;
		untrack(() => void load(id));
	});

	async function load(id: string): Promise<void> {
		if (id === '') return;

		loading = true;
		saved = false;
		issues = [];
		const result = await getQuotaEndpoint(id);
		loading = false;

		if (!result.ok) {
			// Nothing is known about the stored cap after a failed read, so the fields go back to their
			// unread state rather than showing the last answer as if it were current.
			error = result.error.message;
			read = undefined;
			draft = { cost: '', tokens: '' };
			return;
		}

		error = null;
		read = result.data.cap;
		draft = quotaCapForm(result.data.cap);
	}

	async function submit(): Promise<void> {
		const parsed = schemaQuotaCapForm.safeParse(draft);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			return;
		}

		issues = [];
		saving = true;
		const result = await replaceQuotaCap(selected, buildQuotaCapBody(parsed.data));

		if (!result.ok) {
			saving = false;
			error = result.error.message;
			return;
		}

		// The write answers the stored cap, and the read after it is what proves the replacement: a field
		// the body omitted is only known to be gone by asking again. The button stays in its saving state
		// across that read, because the operator asked for one action and what they read is its outcome.
		await load(selected);
		saving = false;
		saved = true;
	}
</script>

<div class="flex flex-col gap-3">
	{#if options.length === 0}
		<!-- No link to Endpoint & Key here: this state means the window table is empty too (its endpoint
		     ids feed the options), so the screen's own empty state is on screen right above and carries
		     that link already. -->
		<StateMessage
			kind="empty"
			title="No endpoints to cap"
			description={labelsUnread
				? 'The endpoint list could not be read, so there is nothing to choose here. Refresh now reads it again.'
				: 'A cap belongs to an upstream endpoint. Add one on Endpoint & Key, then set its budget here.'}
		/>
	{:else}
		<label class="flex w-fit flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Endpoint</span>
			<select
				bind:value={selected}
				class="min-h-11 w-72 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			>
				<option value="">Choose an endpoint</option>
				{#each options as option (option.id)}
					<option value={option.id}>{option.label}</option>
				{/each}
			</select>
		</label>

		{#if selected !== ''}
			<form
				class="flex flex-col gap-3"
				onsubmit={(event) => {
					event.preventDefault();
					void submit();
				}}
			>
				<div class="flex flex-wrap items-end gap-3">
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-[var(--color-text-muted)]">Monthly cost (USD)</span>
						<!-- Read-only until the stored cap has been read: a field the operator could type into
						     before the answer landed would be overwritten by it. An edit also drops the save
						     confirmation, because the sentence below then describes a value the form no
						     longer holds. -->
						<input
							bind:value={draft.cost}
							inputmode="decimal"
							disabled={read === undefined}
							oninput={() => (saved = false)}
							class="min-h-11 w-40 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 disabled:opacity-50"
							placeholder="25"
						/>
					</label>
					<label class="flex flex-col gap-1 text-sm">
						<span class="text-[var(--color-text-muted)]">Monthly tokens</span>
						<input
							bind:value={draft.tokens}
							inputmode="numeric"
							disabled={read === undefined}
							oninput={() => (saved = false)}
							class="min-h-11 w-40 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 disabled:opacity-50"
							placeholder="1000000"
						/>
					</label>
					<button
						type="submit"
						class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
						disabled={saving || read === undefined}
					>
						{saving ? 'Saving' : 'Save the cap'}
					</button>
				</div>

				<p class="text-sm text-[var(--color-text-muted)]">
					Saving replaces both caps at once: an empty field clears that cap. {QUOTA_CAP_WARNING}
				</p>
			</form>

			<!-- The stored state, kept on screen across the read that follows a save: blanking it would
			     unmount the sentence that reports the save. Its name is what tells a screen reader which
			     region this is, since the screen announces more than one. -->
			{#if read !== undefined}
				<p role="status" aria-label="Stored cap" class="text-sm">
					{saved ? 'Saved. ' : ''}{quotaCapSummary(read)}
					{#if read?.updated_at}
						<span class="text-[var(--color-text-muted)]"
							>Stored {formatTimestamp(read.updated_at)}.</span
						>
					{/if}
				</p>
			{:else if loading}
				<p role="status" class="text-sm text-[var(--color-text-muted)]">Reading the stored cap.</p>
			{/if}

			{#if error}
				<p role="alert" class="text-sm text-[var(--color-danger)]">{error}</p>
			{/if}

			<FormIssues {issues} />
		{/if}
	{/if}
</div>
