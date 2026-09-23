<script lang="ts">
	// The fields the Add API Key dialog asks for, one panel per mode
	// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
	//
	// Split out because the dialog holding both panels, their copy, and the submit crosses the panel's line
	// limit. This component owns what the operator types and nothing else; what a submit does with it stays
	// in the dialog, so the wire shape has one home.
	let {
		mode,
		name = $bindable(),
		keyValue = $bindable(),
		priority = $bindable(),
		pasteText = $bindable(),
		rowIssues,
		onedit,
		maxConnections
	}: {
		mode: string;
		name: string;
		keyValue: string;
		priority: string;
		pasteText: string;
		/** The pasted lines a submit refused, keyed by the line they came from. */
		rowIssues: Record<number, string>;
		/** Any edit clears a verdict from a paste the operator has since changed. */
		onedit: () => void;
		maxConnections: number;
	} = $props();

	// The reference's own example (`AddApiKeyModal.js:9`). A template literal rather than a quoted
	// attribute: the example is three lines, and `\n` inside an attribute value would reach the field as a
	// backslash and an n.
	const PASTE_PLACEHOLDER = `name1|sk-key1
name2|sk-key2
sk-key-only-auto-named`;

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2';
</script>

{#if mode === 'bulk'}
	<p class="text-sm text-[var(--color-text-muted)]">
		One key per line. Format: <span class="font-mono">name|apiKey</span> or just
		<span class="font-mono">apiKey</span> (auto-named by index).
	</p>
	<label class="flex flex-col gap-1">
		<span class="text-[var(--color-text-muted)]">Keys</span>
		<textarea
			bind:value={pasteText}
			oninput={onedit}
			rows="6"
			aria-describedby={Object.keys(rowIssues).length > 0 ? 'bulk-key-issues' : undefined}
			class="rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-sm"
			placeholder={PASTE_PLACEHOLDER}></textarea>
	</label>
	{#if Object.keys(rowIssues).length > 0}
		<div id="bulk-key-issues" class="flex flex-col gap-1">
			{#each Object.entries(rowIssues) as [line, message] (line)}
				<p class="text-sm text-[var(--color-danger)]">Line {line}: {message}</p>
			{/each}
		</div>
	{/if}
	<p class="text-sm text-[var(--color-text-muted)]">
		Each line becomes a connection of this provider, under the name that line gives it. Up to
		{maxConnections} keys at a time.
	</p>
{:else}
	<label class="flex flex-col gap-1">
		<span class="text-[var(--color-text-muted)]">Name</span>
		<input bind:value={name} placeholder="Production Key" class={fieldClass} />
	</label>
	<label class="flex flex-col gap-1">
		<span class="text-[var(--color-text-muted)]">API Key</span>
		<input bind:value={keyValue} type="password" autocomplete="off" class={fieldClass} />
	</label>
	<label class="flex flex-col gap-1">
		<span class="text-[var(--color-text-muted)]">Priority</span>
		<input bind:value={priority} inputmode="numeric" class={`${fieldClass} tabular-nums`} />
	</label>
{/if}
