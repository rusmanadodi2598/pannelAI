<script lang="ts">
	// Settings, Logging tab (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	// PATCH /api/v1/settings, the logging group.
	//
	// Capture has a privacy cost: with it on, every request body the gateway records is stored with the
	// request. The toggle carries that warning rather than hiding it behind a tooltip, and the other
	// three fields shape how much is stored and how long it stays.
	//
	// No native `min` attributes, for the same reason the Routing tab has none: a browser bound blocks
	// the submit before Zod sees the value, so the message would be the browser's rather than the
	// schema's. The floors live in the schema and the hint text.
	import { untrack } from 'svelte';
	import { patchLoggingSettings } from '$lib/api/settings';
	import {
		schemaLoggingSettingsForm,
		settingsGroupDirty,
		type LoggingSettingsForm
	} from '$lib/schemas/settings';

	type Props = {
		loaded: LoggingSettingsForm;
		/** Asks the page to re-read the whole settings document after a successful write (§8.6.3). */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	const initial = untrack(() => ({ ...loaded }));
	let draft = $state<LoggingSettingsForm>({ ...initial });
	let server = $state<LoggingSettingsForm>({ ...initial });

	let message = $state<string | null>(null);
	let saving = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	async function save(): Promise<void> {
		message = null;
		const parsed = schemaLoggingSettingsForm.safeParse(draft);
		if (!parsed.success) {
			message = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchLoggingSettings(parsed.data);
		saving = false;

		if (!result.ok) {
			message = result.error.message;
			return;
		}

		const saved = { ...result.data.logging };
		server = saved;
		draft = saved;
		message = 'Saved.';
		await onrefresh();
	}

	function discard(): void {
		draft = { ...server };
		message = null;
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4">
	<h2 class="text-sm font-semibold">Logging</h2>

	<label class="flex items-start gap-3 text-sm">
		<input type="checkbox" bind:checked={draft.request_capture_enabled} class="mt-1" />
		<span>
			Capture request and response bodies
			<span class="block text-[var(--color-text-muted)]">
				With this on, the gateway stores the full request and response body with every logged
				request, which can include sensitive content. With it off, it records the outcome only.
			</span>
		</span>
	</label>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-retention-days">Retention days</label>
		<input
			id="settings-retention-days"
			type="number"
			bind:value={draft.retention_days}
			aria-describedby="settings-retention-days-help"
			class={fieldClass}
		/>
		<span id="settings-retention-days-help" class="text-xs text-[var(--color-text-muted)]">
			How long a captured log row stays before the retention worker purges it. At least 1.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-capture-bytes">Captured body limit, in bytes</label>
		<input
			id="settings-capture-bytes"
			type="number"
			bind:value={draft.capture_body_max_bytes}
			aria-describedby="settings-capture-bytes-help"
			class={fieldClass}
		/>
		<span id="settings-capture-bytes-help" class="text-xs text-[var(--color-text-muted)]">
			A body longer than this is stored up to this point, with a marker saying it was cut. At least
			1.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-console-records">Console buffer size</label>
		<input
			id="settings-console-records"
			type="number"
			bind:value={draft.observability_max_records}
			aria-describedby="settings-console-records-help"
			class={fieldClass}
		/>
		<span id="settings-console-records-help" class="text-xs text-[var(--color-text-muted)]">
			How many lines the console ring buffer holds before older lines fall out. At least 1.
		</span>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={save}>Save logging settings</button
		>
		{#if dirty}
			<button type="button" class="min-h-11 underline" onclick={discard}>Discard changes</button>
			<span class="text-sm font-medium text-[var(--color-text-muted)]">Unsaved changes</span>
		{/if}
		{#if message}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">{message}</p>
		{/if}
	</div>
</div>
