<script lang="ts">
	// Settings, Routing tab (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	// PATCH /api/v1/settings, the routing group.
	//
	// These are the defaults new combos and new routing decisions start from; a value set on a combo
	// wins over these, and the tab says so. The form validates against the API's own floors (each limit
	// is at least 1), so a typo is corrected here rather than as a failed round trip.
	//
	// No native `min` attribute: a browser bound blocks the submit before Zod sees the value, so the
	// message the operator reads would be the browser's and in the browser's language, which is a second
	// validator the panel cannot keep in English.
	import { untrack } from 'svelte';
	import { patchRoutingSettings } from '$lib/api/settings';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		COMBO_STRATEGIES,
		COMBO_STRATEGY_LABELS,
		schemaRoutingSettingsForm,
		settingsGroupDirty,
		type RoutingSettingsForm
	} from '$lib/schemas/settings';

	type Props = {
		/** The routing group as the page last read it. */
		loaded: RoutingSettingsForm;
		/** Asks the page to re-read the whole settings document after a successful write (§8.6.3). */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	// `draft` is what the operator edits; `server` is the last value the gateway confirmed. Keeping both
	// is what makes the dirty indicator honest after a save: the response replaces `server`, so the tab
	// stops reporting an unsaved change the moment the gateway has it.
	// `untrack` marks the initial read as deliberate: the prop supplies the first value, and the save
	// response is what updates it afterwards.
	const initial = untrack(() => ({ ...loaded }));
	let draft = $state<RoutingSettingsForm>({ ...initial });
	let server = $state<RoutingSettingsForm>({ ...initial });

	let message = $state<string | null>(null);
	let saving = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	// §8.4.4: the shared guard asks before a navigation takes this draft away.
	$effect(() => registerDirtyForm(() => dirty));

	async function save(): Promise<void> {
		message = null;
		const parsed = schemaRoutingSettingsForm.safeParse(draft);
		if (!parsed.success) {
			message = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchRoutingSettings(parsed.data);
		saving = false;

		if (!result.ok) {
			message = result.error.message;
			return;
		}

		// The response is the whole stored document, so its routing group is what the gateway kept. Both
		// copies take it: the draft stops being dirty, and Discard returns to the stored value rather
		// than to a stale read (§8.6.3).
		const saved = { ...result.data.routing };
		server = saved;
		draft = saved;
		message = 'Saved. These defaults apply to new combos and new routing decisions only.';
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
	<h2 class="text-sm font-semibold">Routing</h2>
	<p class="text-sm text-[var(--color-text-muted)]">
		Defaults for new combos and for routing decisions. A value set on a combo wins over these.
	</p>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-combo-strategy">Default combo strategy</label>
		<select
			id="settings-combo-strategy"
			value={draft.combo_strategy}
			onchange={(event) =>
				(draft.combo_strategy = event.currentTarget.value as RoutingSettingsForm['combo_strategy'])}
			class={fieldClass}
		>
			{#each COMBO_STRATEGIES as strategy (strategy)}
				<option value={strategy}>{COMBO_STRATEGY_LABELS[strategy]}</option>
			{/each}
		</select>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-combo-sticky">Combo sticky limit</label>
		<input
			id="settings-combo-sticky"
			type="number"
			bind:value={draft.combo_sticky_limit}
			aria-describedby="settings-combo-sticky-help"
			class={fieldClass}
		/>
		<span id="settings-combo-sticky-help" class="text-xs text-[var(--color-text-muted)]">
			How many requests a round-robin combo keeps on one model before rotating. At least 1.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-routing-sticky">Routing sticky limit</label>
		<input
			id="settings-routing-sticky"
			type="number"
			bind:value={draft.sticky_limit}
			aria-describedby="settings-routing-sticky-help"
			class={fieldClass}
		/>
		<span id="settings-routing-sticky-help" class="text-xs text-[var(--color-text-muted)]">
			How many consecutive requests one upstream endpoint serves before round-robin rotation. At
			least 1.
		</span>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={save}>Save routing settings</button
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
