<script lang="ts">
	// Copy control (docs/SPEC-UI/001-SPEC-UI.md §6.12, §6.10).
	//
	// The value is the caller's, so the placeholder rule lives with the text that carries it and this
	// control never composes a credential. The label is a prop because what is copied differs by screen:
	// a URL on the API Docs screen, an install line on the Skills screen.
	//
	// The three sentences below belong to the control rather than to a screen, which is why they live
	// here: they describe a clipboard write failing or succeeding, and every caller reads the same either
	// way. They moved out of `strings/api-docs.ts` when Skills became the second caller.
	//
	// Two write paths, because the first one is missing exactly where this panel is usually opened. The
	// async clipboard API exists only in a secure context, and the panel is normally reached at an
	// `http://<host>:3000` address, which is not one; a browser can refuse a write even where it exists.
	// The selection-based path is tried second, and only when both fail does the control report a
	// failure.
	//
	// A caller that has to act on the outcome can pass `oncopied`, which fires only after a write
	// succeeded. The one-time key modal uses it to unlock dismissal, so the modal opens up when the key
	// actually reached the clipboard rather than when the button was pressed.
	const DEFAULT_LABEL = 'Copy';
	const COPIED = 'Copied.';
	const FAILED = 'Copy failed. Select the text and copy it.';

	let {
		value,
		label = DEFAULT_LABEL,
		oncopied
	}: { value: string; label?: string; oncopied?: () => void } = $props();

	let state = $state<'idle' | 'copied' | 'failed'>('idle');

	/** The async clipboard API, when the context is secure enough to have one. */
	async function writeModern(text: string): Promise<boolean> {
		if (typeof navigator === 'undefined' || !navigator.clipboard) return false;
		try {
			await navigator.clipboard.writeText(text);
			return true;
		} catch {
			return false;
		}
	}

	/**
	 * The selection-based copy, for an origin without the async API.
	 *
	 * The textarea is off screen rather than hidden, because a node with `display: none` cannot be
	 * selected, and it is removed again whether the copy worked or not. `execCommand` is deprecated and
	 * is still the only path that works on an insecure origin, which is why it is here rather than
	 * dropped.
	 */
	function writeLegacy(text: string): boolean {
		const area = document.createElement('textarea');
		area.value = text;
		area.setAttribute('readonly', '');
		area.style.position = 'fixed';
		area.style.top = '0';
		area.style.opacity = '0';
		document.body.appendChild(area);

		try {
			area.select();
			area.setSelectionRange(0, text.length);
			return document.execCommand('copy');
		} catch {
			return false;
		} finally {
			area.remove();
		}
	}

	async function write(): Promise<void> {
		if ((await writeModern(value)) || writeLegacy(value)) {
			state = 'copied';
			oncopied?.();
			return;
		}

		state = 'failed';
	}
</script>

<span class="inline-flex items-center gap-2">
	<button
		type="button"
		class="min-h-9 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2.5 text-xs"
		onclick={() => void write()}>{label}</button
	>

	<span class="text-xs text-[var(--color-text-muted)]" role="status" aria-live="polite">
		{#if state === 'copied'}
			{COPIED}
		{:else if state === 'failed'}
			{FAILED}
		{/if}
	</span>
</span>
