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
	//
	// Two groups of the form are components: the park windows and the failure-class table, because a tab
	// that wrote both would spend its line budget on markup rather than on the draft it owns.
	import { untrack } from 'svelte';
	import SettingsFailureClassesEditor from '$lib/components/SettingsFailureClassesEditor.svelte';
	import SettingsKeyParkFields from '$lib/components/SettingsKeyParkFields.svelte';
	import SettingsSaveBar from '$lib/components/SettingsSaveBar.svelte';
	import { patchRoutingSettings } from '$lib/api/settings';
	import type { ApiError } from '$lib/api/errors';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		buildRoutingSettingsBody,
		schemaRoutingSettingsForm,
		type RoutingSettings,
		type RoutingSettingsForm
	} from '$lib/schemas/settings';
	import { cloneRoutingDraft, routingDraftFromSettings } from '$lib/schemas/settings-routing-draft';
	import { settingsGroupDirty } from '$lib/schemas/settings-dirty';
	import {
		COMBO_STRATEGIES,
		COMBO_STRATEGY_LABELS,
		CREDENTIAL_ROTATIONS,
		CREDENTIAL_ROTATION_LABELS
	} from '$lib/schemas/settings-strategies';

	type Props = {
		/** The routing group as the page last read it, before this tab narrows it to its own keys. */
		loaded: RoutingSettings;
		/** Asks the page to re-read the whole settings document after a successful write (§8.6.3). */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	// `draft` is what the operator edits; `server` is the last value the gateway confirmed. Keeping both
	// is what makes the dirty indicator honest after a save: the response replaces `server`, so the tab
	// stops reporting an unsaved change the moment the gateway has it.
	// `untrack` marks the initial read as deliberate: the prop supplies the first value, and the save
	// response is what updates it afterwards. The narrowing and the copying are the routing draft's own
	// rules and live in `settings-routing-draft`.
	const initial = untrack(() => routingDraftFromSettings(loaded));
	let draft = $state<RoutingSettingsForm>(initial);
	let server = $state<RoutingSettingsForm>(cloneRoutingDraft(initial));

	let message = $state<string | ApiError | null>(null);
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
		const result = await patchRoutingSettings(buildRoutingSettingsBody(parsed.data));
		saving = false;

		if (!result.ok) {
			message = result.error;
			return;
		}

		// The response is the whole stored document, so its routing group is what the gateway kept. Both
		// copies take it: the draft stops being dirty, and Discard returns to the stored value rather
		// than to a stale read (§8.6.3). The draft gets a fresh copy so a later edit cannot touch server.
		const saved = routingDraftFromSettings(result.data.routing);
		server = saved;
		draft = cloneRoutingDraft(saved);
		message = 'Saved. Applies to new combos and decisions.';
		await onrefresh();
	}

	function discard(): void {
		draft = cloneRoutingDraft(server);
		message = null;
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4">
	<h2 class="text-sm font-semibold">Routing</h2>
	<p class="text-sm text-[var(--color-text-muted)]">
		Defaults for new combos and decisions. A combo value wins.
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
			Requests kept on one model before rotating. At least 1.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-credential-rotation">Credential rotation</label>
		<select
			id="settings-credential-rotation"
			value={draft.fallback_strategy}
			onchange={(event) =>
				(draft.fallback_strategy = event.currentTarget
					.value as RoutingSettingsForm['fallback_strategy'])}
			aria-describedby="settings-credential-rotation-help"
			class={fieldClass}
		>
			{#each CREDENTIAL_ROTATIONS as rotation (rotation)}
				<option value={rotation}>{CREDENTIAL_ROTATION_LABELS[rotation]}</option>
			{/each}
		</select>
		<span id="settings-credential-rotation-help" class="text-xs text-[var(--color-text-muted)]">
			Fill-first: first usable account. Round-robin rotates. A provider can override.
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
			Requests per account before rotating. Round-robin only. At least 1.
		</span>
	</div>

	<SettingsKeyParkFields bind:park={draft.key_park} />

	<SettingsFailureClassesEditor bind:classes={draft.failure_classes} />

	<h3 class="pt-1 text-sm font-semibold">Failover budget</h3>
	<p class="text-sm text-[var(--color-text-muted)]">
		How far one client request may go trying credentials before it gives up.
	</p>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-failover-total">Failover time budget</label>
		<input
			id="settings-failover-total"
			type="number"
			bind:value={draft.failover.total_ms}
			aria-describedby="settings-failover-total-help"
			class={fieldClass}
		/>
		<span id="settings-failover-total-help" class="text-xs text-[var(--color-text-muted)]">
			Milliseconds one request spends failing over before it stops. 1000 to 120000.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-failover-attempts">Failover attempt cap</label>
		<input
			id="settings-failover-attempts"
			type="number"
			bind:value={draft.failover.max_attempts}
			aria-describedby="settings-failover-attempts-help"
			class={fieldClass}
		/>
		<span id="settings-failover-attempts-help" class="text-xs text-[var(--color-text-muted)]">
			Credentials one request may try before it stops. 1 to 200.
		</span>
	</div>

	<SettingsSaveBar
		saveLabel="Save routing settings"
		{saving}
		{dirty}
		{message}
		onsave={() => void save()}
		ondiscard={discard}
	/>
</div>
