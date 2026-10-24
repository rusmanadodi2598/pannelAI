<script lang="ts">
	// Batch add for the proxy pool (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// The paste is a dialog, the same shape the bulk credential add already takes: the operator leaves
	// the pool table, does one thing, and comes back. The page mounts this only while it is open, so
	// closing it discards the paste rather than holding a draft nobody can see.
	//
	// The paste is parsed in the panel and previewed before anything is sent, and the raw text is
	// never submitted: what goes out is one create request per parsed row. A line that cannot be read
	// is listed with its line number and the reason, so the operator can fix the paste rather than
	// guess which of nine lines was dropped.
	//
	// The rows are submitted one at a time, in order, and each one reports its own outcome. A refusal
	// on the third row does not silently drop the rest, and when the run finishes the box keeps only
	// the lines that did not make it, so pressing the button again cannot add a duplicate of a
	// candidate that already landed.
	//
	// The preview says whether a password came with the line rather than the value. A preview is a
	// place a credential would sit in clear text and unmasked, and the row's label is the parsed
	// address, so the Address column carries both facts.
	import { createProxy } from '$lib/api/proxies';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import {
		parseProxyLines,
		proxyCreateBody,
		type ParsedProxyRow,
		type RejectedProxyLine
	} from '$lib/schemas/proxy-batch';
	import { PROXY_PROTOCOL_LABELS } from '$lib/schemas/proxy';

	let {
		onsubmitted,
		onclose
	}: {
		/** Re-reads the pool after a run, so the table shows what the gateway stored (§8.6.3). */
		onsubmitted: () => Promise<void>;
		onclose: () => void;
	} = $props();

	let text = $state('');
	let submitting = $state(false);
	// Keyed by the pasted line rather than by its number. When a run leaves only the refused lines in
	// the box, the numbering restarts at 1, so a number-keyed outcome would lose the reason it was
	// holding. The raw line is stable across that rewrite, and a refused host is refused every time, so
	// two identical lines cannot disagree about their outcome.
	let outcomes = $state<Record<string, { ok: boolean; message: string | ApiError }>>({});
	let summary = $state<string | null>(null);

	const parsed = $derived(parseProxyLines(text));

	// "Add 1 proxies" is the kind of copy a table-driven count produces by accident, so the singular
	// is stated rather than derived from a plural template.
	const submitLabel = $derived(
		submitting
			? 'Adding'
			: parsed.rows.length === 1
				? 'Add 1 proxy'
				: `Add ${parsed.rows.length} proxies`
	);

	async function submit(): Promise<void> {
		outcomes = {};
		summary = null;
		submitting = true;

		const rows = parsed.rows;
		const failed: string[] = [];
		let added = 0;

		// Sequential on purpose: the run reports one outcome per row in paste order, and a row that
		// fails is a row the operator has to read before deciding about the next one.
		for (const row of rows) {
			const answer = await createProxy(proxyCreateBody(row));

			if (answer.ok) {
				added += 1;
				outcomes[row.raw] = { ok: true, message: 'Added' };
				continue;
			}

			outcomes[row.raw] = { ok: false, message: answer.error };
			failed.push(row.raw);
		}

		submitting = false;
		text = failed.join('\n');
		summary =
			failed.length === 0
				? `Added ${added} of ${rows.length}.`
				: `Added ${added} of ${rows.length}. Refused lines stay here, with the reason.`;

		await onsubmitted();
	}

	function rejectionText(rejection: RejectedProxyLine): string {
		return `Line ${rejection.line}: ${rejection.reason}`;
	}

	/** What the line carried, in words, with no secret spelled out. */
	function credentialText(row: ParsedProxyRow): string {
		const who = row.username === '' ? 'no username' : row.username;
		const secret = row.password === '' ? 'no password' : 'password set';
		return `${who}, ${secret}`;
	}

	// Two example lines rather than a rule: the format is easier to read from an example, and this is
	// the one place a placeholder carries content rather than a field name. The credential words are
	// the field names, so the example never reads like a real account.
	const PASTE_PLACEHOLDER = `http://proxy.example.com:8080
socks5://username:password@proxy.example.com:1080`;

	// An edit to the paste clears the previous run's outcomes and summary. Without this, retyping a line
	// that was refused once would show a stale reason beside a row nobody has sent yet. A programmatic
	// `text` assignment does not fire an input event, so the run's own rewrite is unaffected.
	function forgetRun(): void {
		outcomes = {};
		summary = null;
	}
</script>

<Modal title="Add several at once" open {onclose} wide>
	<div class="flex flex-col gap-4">
		<p class="text-sm text-[var(--color-text-muted)]">
			Paste one proxy URL per line, then review the preview.
		</p>

		<div class="flex flex-col gap-1 text-sm">
			<label for="proxy-batch-text">Proxy URLs, one per line</label>
			<textarea
				id="proxy-batch-text"
				rows="6"
				bind:value={text}
				oninput={forgetRun}
				placeholder={PASTE_PLACEHOLDER}
				aria-describedby="proxy-batch-help"
				class="rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-2 font-mono text-sm"
			></textarea>
			<span id="proxy-batch-help" class="text-xs text-[var(--color-text-muted)]">
				http, https, or socks5. Credentials go in the URL.
			</span>
		</div>

		{#if text.trim() !== ''}
			<p class="text-sm">{parsed.rows.length} ready, {parsed.rejected.length} rejected.</p>
		{/if}

		{#if summary}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">{summary}</p>
		{/if}

		{#if parsed.rejected.length > 0}
			<ul class="flex flex-col gap-1 text-sm">
				{#each parsed.rejected as rejection (rejection.line)}
					<li class="text-[var(--color-danger)]">{rejectionText(rejection)}</li>
				{/each}
			</ul>
		{/if}

		{#if parsed.rows.length > 0}
			<div
				class="mobile-key-value-frame overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
			>
				<table class="mobile-key-value-table w-full border-collapse text-sm">
					<caption class="sr-only">What the paste parsed into, before anything is sent</caption>
					<thead class="bg-[var(--color-surface-2)] text-left">
						<tr>
							<th scope="col" class="px-3 py-2 font-medium">Line</th>
							<th scope="col" class="px-3 py-2 font-medium">Protocol</th>
							<th scope="col" class="px-3 py-2 font-medium">Address</th>
							<th scope="col" class="px-3 py-2 font-medium">Credential</th>
							<th scope="col" class="px-3 py-2 font-medium">Result</th>
						</tr>
					</thead>
					<tbody>
						{#each parsed.rows as row (row.line)}
							{@const outcome = outcomes[row.raw]}
							<tr class="border-t border-[var(--color-border)]">
								<td data-label="Line" class="px-3 py-2 tabular-nums">{row.line}</td>
								<td data-label="Protocol" class="px-3 py-2">
									{PROXY_PROTOCOL_LABELS[row.protocol]}
								</td>
								<td data-label="Address" class="px-3 py-2 break-words">
									{row.host}:{row.port}
								</td>
								<td data-label="Credential" class="px-3 py-2 break-words">
									{credentialText(row)}
								</td>
								<td data-label="Result" class="px-3 py-2">
									{#if outcome}
										<span class={outcome.ok ? '' : 'text-[var(--color-danger)]'}>
											<ApiErrorText error={outcome.message} />
										</span>
									{:else}
										<span class="text-[var(--color-text-muted)]">Not sent</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<div class="flex flex-wrap items-center gap-3">
				<Button
					variant="ghost"
					class="underline"
					onclick={() => {
						text = '';
						forgetRun();
					}}
				>
					Clear the paste
				</Button>
			</div>
		{/if}
	</div>

	{#snippet footer()}
		<Button variant="ghost" class="underline" onclick={onclose}>Cancel</Button>
		<Button
			class="px-4"
			disabled={submitting || parsed.rows.length === 0}
			aria-busy={submitting}
			onclick={() => void submit()}
		>
			{submitLabel}
		</Button>
	{/snippet}
</Modal>
