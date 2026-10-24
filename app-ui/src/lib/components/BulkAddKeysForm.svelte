<script lang="ts">
	// The repeatable row mode for adding several keys in one submit (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// The batch is all-or-nothing (SPEC-API §8.1), so this screen has to say two things a per-row form would
	// not: nothing was stored when any row is refused, and which row was refused. The refusal's own rows are
	// read rather than reconstructed, so the message that lands on a row is the server's, and every row of a
	// refused batch is kept on screen because the operator has to fix one of them.
	//
	// An outcome describes the batch that was submitted, not the rows on screen now, so any edit clears it:
	// a message left over from a batch the operator has since changed would point at the wrong row.
	import { addEndpointKeys } from '$lib/api/endpoints';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { Button } from '$lib/primitives/button/index.js';
	import {
		MAX_KEYS_PER_ENDPOINT,
		schemaAddEndpointKeyForm,
		schemaBulkAddKeysForm
	} from '$lib/schemas/endpoint-credential';
	import type { BulkRefusal } from '$lib/schemas/endpoint-bulk';

	let { endpointId, onadded }: { endpointId: string; onadded: () => void } = $props();

	type Row = { label: string; value: string };
	type Outcome = { summary: string; rowMessages: Record<number, string>; apiError?: ApiError };

	const formId = $props.id();
	const rowErrorId = (index: number): string => `${formId}-row-${index}`;
	// §8.8.5: the hint is described by the field, not part of its name, so a screen reader announces the
	// field as "Label for row 2" and then the rule. The wrapping-label pattern would fold "(optional)" into
	// the name.
	const fieldId = (kind: 'label' | 'key', index: number): string => `${formId}-${kind}-${index}`;

	let rows = $state<Row[]>([{ label: '', value: '' }]);
	let submitting = $state(false);
	let outcome = $state<Outcome | null>(null);
	const atLimit = $derived(rows.length >= MAX_KEYS_PER_ENDPOINT);
	const limitNote = $derived(
		atLimit
			? `The limit is ${MAX_KEYS_PER_ENDPOINT} keys per submit.`
			: `Up to ${MAX_KEYS_PER_ENDPOINT} keys per submit.`
	);

	function clear(): void {
		outcome = null;
	}

	function addRow(): void {
		rows = [...rows, { label: '', value: '' }];
		clear();
	}

	function removeRow(index: number): void {
		rows = rows.filter((_, position) => position !== index);
		clear();
	}

	// The server's own row verdicts, keyed by the index it reported.
	function messagesFrom(refusal: BulkRefusal | undefined): Record<number, string> {
		const messages: Record<number, string> = {};
		for (const row of refusal?.results ?? []) {
			if (row.error) messages[row.index] = row.error;
		}
		return messages;
	}

	async function submit(): Promise<void> {
		const rowMessages: Record<number, string> = {};
		const keys: { label?: string; value: string }[] = [];

		rows.forEach((row, index) => {
			const parsed = schemaAddEndpointKeyForm.safeParse({
				label: row.label === '' ? undefined : row.label,
				value: row.value
			});
			if (!parsed.success) {
				rowMessages[index] = parsed.error.issues[0]?.message ?? 'That key cannot be added.';
				return;
			}
			keys.push(parsed.data);
		});

		if (Object.keys(rowMessages).length > 0) {
			outcome = { summary: 'Nothing was sent: fix the rows marked below.', rowMessages };
			return;
		}

		const batch = schemaBulkAddKeysForm.safeParse({ keys });
		if (!batch.success) {
			outcome = {
				summary: batch.error.issues[0]?.message ?? 'That batch cannot be sent.',
				rowMessages: {}
			};
			return;
		}

		submitting = true;
		const result = await addEndpointKeys(endpointId, batch.data);
		submitting = false;

		if (!result.ok) {
			outcome = {
				summary: 'Nothing was added:',
				rowMessages: messagesFrom(result.refusal),
				apiError: result.error
			};
			return;
		}

		const added = result.data.created.length;
		outcome = {
			summary: added === 1 ? '1 key was added.' : `${added} keys were added.`,
			rowMessages: {}
		};
		rows = [{ label: '', value: '' }];
		onadded();
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2';
</script>

<div class="flex flex-col gap-2">
	<span class="font-medium">Add several keys</span>
	<p class="text-sm text-[var(--color-text-muted)]">One submit adds all rows or none.</p>

	{#if outcome}
		<p role={outcome.apiError ? 'alert' : 'status'} class="text-sm">
			{outcome.summary}
			{#if outcome.apiError}<ApiErrorText error={outcome.apiError} prefix=" " />{/if}
		</p>
	{/if}

	{#each rows as row, index (index)}
		<div class="flex flex-col gap-1">
			<div class="flex flex-wrap items-end gap-3">
				<div class="flex flex-col gap-1">
					<span class="flex items-baseline gap-1">
						<label for={fieldId('label', index)} class="text-[var(--color-text-muted)]"
							>Label for row {index + 1}</label
						>
						<span id={`${formId}-hint-${index}`} class="text-sm text-[var(--color-text-muted)]"
							>(optional)</span
						>
					</span>
					<input
						id={fieldId('label', index)}
						bind:value={row.label}
						oninput={clear}
						aria-describedby={`${formId}-hint-${index}`}
						class={fieldClass}
					/>
				</div>
				<div class="flex flex-col gap-1">
					<label for={fieldId('key', index)} class="text-[var(--color-text-muted)]"
						>Key for row {index + 1}</label
					>
					<input
						id={fieldId('key', index)}
						bind:value={row.value}
						type="password"
						autocomplete="off"
						oninput={clear}
						aria-describedby={outcome?.rowMessages[index] ? rowErrorId(index) : undefined}
						class={fieldClass}
					/>
				</div>
				<Button
					variant="ghost"
					class="underline"
					disabled={rows.length === 1}
					onclick={() => removeRow(index)}>Remove row {index + 1}</Button
				>
			</div>

			{#if outcome?.rowMessages[index]}
				<p id={rowErrorId(index)} class="text-sm text-[var(--color-danger)]">
					{outcome.rowMessages[index]}
				</p>
			{/if}
		</div>
	{/each}

	<div class="flex flex-wrap items-center gap-3">
		<Button variant="outline" disabled={atLimit} onclick={addRow}>Add another row</Button>

		<Button variant="default" disabled={submitting} aria-busy={submitting} onclick={submit}>
			{submitting ? 'Adding keys' : rows.length === 1 ? 'Add 1 key' : `Add ${rows.length} keys`}
		</Button>

		<span class="text-sm text-[var(--color-text-muted)]">{limitNote}</span>
	</div>
</div>
