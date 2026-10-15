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
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import SettingsFailureClassesEditor from '$lib/components/SettingsFailureClassesEditor.svelte';
	import { patchRoutingSettings } from '$lib/api/settings';
	import type { ApiError } from '$lib/api/errors';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		COMBO_STRATEGIES,
		COMBO_STRATEGY_LABELS,
		CREDENTIAL_ROTATIONS,
		CREDENTIAL_ROTATION_LABELS,
		FAILOVER_DEFAULT_MAX_ATTEMPTS,
		FAILOVER_DEFAULT_TOTAL_MS,
		buildRoutingSettingsBody,
		schemaRoutingSettingsForm,
		settingsGroupDirty,
		type RoutingSettings,
		type RoutingSettingsForm
	} from '$lib/schemas/settings';
	import { failureClassesToForm, namedFailureClasses } from '$lib/schemas/settings-failure-classes';

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
	// response is what updates it afterwards.
	//
	// The tab owns the group's global keys and nothing else. `provider_strategies` belongs to the provider
	// screen's own switch, and carrying it here would put it in a strict form that refuses it and in a
	// PATCH body the tab has no business writing. `key_park` windows are narrowed to what is actually
	// stored: a zero or absent window is the gateway default, not a value the operator chose, so it is held
	// blank. The class table follows the same rule from the other side, because the response answers only
	// the classes an operator stored and a class it does not name is still running its documented default.
	const parkSeconds = (value: number | undefined): number | undefined =>
		value && value > 0 ? value : undefined;
	const ownKeys = (group: RoutingSettings): RoutingSettingsForm => ({
		combo_strategy: group.combo_strategy,
		combo_sticky_limit: group.combo_sticky_limit,
		sticky_limit: group.sticky_limit,
		fallback_strategy: group.fallback_strategy,
		key_park: {
			auth_seconds: parkSeconds(group.key_park?.auth_seconds),
			rate_limit_seconds: parkSeconds(group.key_park?.rate_limit_seconds),
			transient_seconds: parkSeconds(group.key_park?.transient_seconds),
			billing_seconds: parkSeconds(group.key_park?.billing_seconds)
		},
		failover: {
			total_ms: group.failover?.total_ms ?? FAILOVER_DEFAULT_TOTAL_MS,
			max_attempts: group.failover?.max_attempts ?? FAILOVER_DEFAULT_MAX_ATTEMPTS
		},
		failure_classes: failureClassesToForm(group.failure_classes)
	});

	// The two copies must be independent objects: `key_park`, `failover` and each class entry are nested, so
	// a shallow copy would hand `draft` and `server` the same sub-object, and editing one would silently
	// edit the other.
	const cloneForm = (form: RoutingSettingsForm): RoutingSettingsForm => ({
		combo_strategy: form.combo_strategy,
		combo_sticky_limit: form.combo_sticky_limit,
		sticky_limit: form.sticky_limit,
		fallback_strategy: form.fallback_strategy,
		key_park: { ...form.key_park },
		failover: { ...form.failover },
		failure_classes: namedFailureClasses(form.failure_classes)
	});

	const initial = untrack(() => ownKeys(loaded));
	let draft = $state<RoutingSettingsForm>(initial);
	let server = $state<RoutingSettingsForm>(cloneForm(initial));

	let message = $state<string | ApiError | null>(null);
	let saving = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	// §8.4.4: the shared guard asks before a navigation takes this draft away.
	$effect(() => registerDirtyForm(() => dirty));

	// A park is blank by design when it inherits, and a number input hands back an empty string,
	// which a coerced field would turn into 0 and refuse. Storing `undefined` for blank keeps inherit as
	// "send nothing", which is exactly what the gateway reads as its own default.
	function setPark(field: keyof RoutingSettingsForm['key_park'], raw: string): void {
		const trimmed = raw.trim();
		draft.key_park[field] = trimmed === '' ? undefined : Number(trimmed);
	}

	// The four park windows share one control shape and differ only in the failure class they park after,
	// so they are data rather than four copies of the same markup. The bound text names what each window
	// actually measures, not the field name.
	const PARK_WINDOWS: ReadonlyArray<{
		field: keyof RoutingSettingsForm['key_park'];
		id: string;
		label: string;
		help: string;
	}> = [
		{
			field: 'auth_seconds',
			id: 'settings-park-auth',
			label: 'Auth park window',
			help: 'Seconds a credential waits after the gateway rejected its key as bad or expired. 1 to 900.'
		},
		{
			field: 'rate_limit_seconds',
			id: 'settings-park-rate',
			label: 'Rate-limit park window',
			help: 'Seconds a credential waits after the upstream answered 429. 1 to 300.'
		},
		{
			field: 'transient_seconds',
			id: 'settings-park-transient',
			label: 'Transient park window',
			help: 'Seconds a credential waits after a temporary upstream fault. 1 to 300.'
		},
		{
			field: 'billing_seconds',
			id: 'settings-park-billing',
			label: 'Billing park window',
			help: 'Seconds a credential waits after a quota or billing refusal. 1 to 3600.'
		}
	];

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
		const saved = ownKeys(result.data.routing);
		server = saved;
		draft = cloneForm(saved);
		message = 'Saved. Applies to new combos and decisions.';
		await onrefresh();
	}

	function discard(): void {
		draft = cloneForm(server);
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

	<h3 class="pt-1 text-sm font-semibold">Key park</h3>
	<p class="text-sm text-[var(--color-text-muted)]">
		How long a credential is set aside after each kind of failure. A parked credential reopens on
		its own when its window passes; blank means the gateway default applies.
	</p>

	{#each PARK_WINDOWS as window (window.field)}
		<div class="flex flex-col gap-1 text-sm">
			<label for={window.id}>{window.label}</label>
			<input
				id={window.id}
				type="number"
				value={draft.key_park[window.field] ?? ''}
				oninput={(event) => setPark(window.field, event.currentTarget.value)}
				aria-describedby={`${window.id}-help`}
				class={fieldClass}
			/>
			<span id={`${window.id}-help`} class="text-xs text-[var(--color-text-muted)]">
				{window.help}
			</span>
		</div>
	{/each}

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
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				<ApiErrorText error={message} />
			</p>
		{/if}
	</div>
</div>
