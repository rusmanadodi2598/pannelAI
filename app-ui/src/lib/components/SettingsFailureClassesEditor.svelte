<script lang="ts">
	// The failure-class table inside the Routing tab (docs/SPEC-API/001-SPEC-API.md §7.5, §7.14).
	//
	// The tab keeps its draft, its dirty answer and its save; this is one section of the form it saves, and
	// six rows of three controls each is more markup than the tab has line budget for (SPEC-UI §10, 250).
	// The rows are the six classes whose failure the credential caused. A class the credential had nothing
	// to do with parks no key, so the gateway refuses an entry for it by name and there is nothing here to
	// edit.
	//
	// An entry the operator has not set does not exist. A blank field writes the class key away rather than
	// leaving an object with three undefined fields in it, because the tab compares its draft to what the
	// gateway returned key by key: an empty object where the response has no key at all is an unsaved
	// change that saves nothing, and an entry that names no field is configuration nobody reads.
	//
	// No native `min` or `max` on the windows, for the same reason the tab gives for its park windows: a
	// browser bound would refuse the value before Zod sees it, and the message would be the browser's.
	import {
		FAILURE_ACTIONS,
		FAILURE_ACTION_HELP,
		FAILURE_ACTION_LABELS,
		failureClassLabel,
		type CredentialFailureClass,
		type FailureAction,
		type FailureClassesForm,
		type FailureClassPolicyForm
	} from '$lib/schemas/settings-failure-classes';

	type Props = {
		/** The class table the routing tab holds as its draft. Writes land in the tab's copy. */
		classes: FailureClassesForm | undefined;
	};

	let { classes = $bindable() }: Props = $props();

	// The class the router starts each row on, with the sentence that says what a failure of that kind is.
	// The name is the wire token, so an operator reading a parked key in the endpoint table finds the same
	// words here.
	const CLASS_ROWS: ReadonlyArray<{ class: CredentialFailureClass; id: string; help: string }> = [
		{
			class: 'RATE_LIMIT_TEMPORARY',
			id: 'rate',
			help: 'A throttle the vendor expects to lift on its own. A Retry-After header is honoured up to the ceiling.'
		},
		{
			class: 'QUOTA_EXHAUSTED',
			id: 'quota',
			help: "An allowance spent for the vendor's own period. The key waits until the reset the vendor states."
		},
		{
			class: 'AUTH_INVALID',
			id: 'auth',
			help: 'A credential the upstream refused as bad or expired, with no quota or billing signal with it.'
		},
		{
			class: 'BILLING_EXHAUSTED',
			id: 'billing',
			help: 'An account with no credit left. Nothing the key can do changes it.'
		},
		{
			class: 'SERVER_TRANSIENT',
			id: 'transient',
			help: 'An upstream outage that says nothing about the credential.'
		},
		{
			class: 'UNKNOWN',
			id: 'unknown',
			help: 'A failure no signature and no status rule recognised.'
		}
	];

	const WINDOW_FIELDS: ReadonlyArray<{
		field: 'ceiling_seconds' | 'default_cooldown_seconds';
		label: string;
		help: string;
	}> = [
		{
			field: 'ceiling_seconds',
			label: 'Ceiling (seconds)',
			help: 'The longest wait the gateway may arrive at by itself, counted from the failure. It does not shorten a reset the vendor stated. 1 to 2592000.'
		},
		{
			field: 'default_cooldown_seconds',
			label: 'Flat cooldown (seconds)',
			help: 'The wait used when the vendor stated none of its own. Blank keeps the class ladder, which is how a rate limit still backs off further each time.'
		}
	];

	// A class that moves on without withholding the credential has no wait to bound, so the two windows are
	// taken off screen rather than disabled: the router reads no window for `rotate` or `stop`. What an
	// operator already typed or stored is kept out of sight instead of deleted, because the class can be
	// put back on `park` in the same sitting, and a ceiling inherited under one action is still the
	// ceiling for the next one.
	function parks(action: FailureAction | undefined): boolean {
		return action === undefined || action === 'park';
	}

	function writeClass(classToken: CredentialFailureClass, next: FailureClassPolicyForm): void {
		const table: FailureClassesForm = { ...classes };
		const merged: FailureClassPolicyForm = { ...table[classToken], ...next };
		const named: FailureClassPolicyForm = {};

		if (merged.action !== undefined) named.action = merged.action;
		if (merged.ceiling_seconds !== undefined) named.ceiling_seconds = merged.ceiling_seconds;
		if (merged.default_cooldown_seconds !== undefined) {
			named.default_cooldown_seconds = merged.default_cooldown_seconds;
		}

		if (Object.keys(named).length > 0) table[classToken] = named;
		else delete table[classToken];

		classes = table;
	}

	function setAction(classToken: CredentialFailureClass, raw: string): void {
		// The select can only answer a word from the list or the empty inherit value, and looking the word up
		// rather than asserting it means a value this build does not know falls back to inherit instead of
		// reaching the gateway as an action it would refuse.
		const action = FAILURE_ACTIONS.find((candidate) => candidate === raw);
		writeClass(classToken, { action });
	}

	function setSeconds(
		classToken: CredentialFailureClass,
		window: 'ceiling_seconds' | 'default_cooldown_seconds',
		raw: string
	): void {
		const trimmed = raw.trim();
		// Blank is inherit. A number input that never saw a keystroke hands back `''`, and a coerced 0 would
		// be sent as a window nobody named and refused as a bound.
		const seconds = trimmed === '' ? undefined : Number(trimmed);

		writeClass(
			classToken,
			window === 'ceiling_seconds'
				? { ceiling_seconds: seconds }
				: { default_cooldown_seconds: seconds }
		);
	}

	const fieldClass =
		'min-h-11 w-full rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-3">
	<h3 class="pt-1 text-sm font-semibold">Failure classes</h3>
	<p class="text-sm text-[var(--color-text-muted)]">
		What the router does once it knows what the upstream's refusal was, per class. Blank keeps the
		documented behaviour. Every provider inherits these until it carries its own entry for a class.
	</p>
	<p id="settings-class-actions" class="text-xs text-[var(--color-text-muted)]">
		{FAILURE_ACTION_HELP.park}
		{FAILURE_ACTION_HELP.rotate}
		{FAILURE_ACTION_HELP.stop}
	</p>
	<p class="text-xs text-[var(--color-text-muted)]">
		Classes the credential did not cause are not listed: an unreachable target, a gate that refuses
		this gateway, or a request refused on its own shape parks no key, so there is no window to tune.
	</p>

	{#each CLASS_ROWS as row (row.class)}
		{@const entry = classes?.[row.class]}
		<div
			class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3"
		>
			<h4 id="settings-class-{row.id}-title" class="text-sm font-medium">
				{failureClassLabel(row.class)}
			</h4>
			<p class="text-xs text-[var(--color-text-muted)]">{row.help}</p>

			<div
				role="group"
				aria-labelledby={`settings-class-${row.id}-title`}
				class="flex flex-wrap items-start gap-3 max-sm:flex-col max-sm:items-stretch"
			>
				<div class="flex w-full flex-col gap-1 text-sm sm:w-52">
					<label for={`settings-class-${row.id}-action`}>Action</label>
					<select
						id={`settings-class-${row.id}-action`}
						value={entry?.action ?? ''}
						onchange={(event) => setAction(row.class, event.currentTarget.value)}
						aria-describedby="settings-class-actions"
						class={fieldClass}
					>
						<option value="">Inherit</option>
						{#each FAILURE_ACTIONS as action (action)}
							<option value={action}>{FAILURE_ACTION_LABELS[action]}</option>
						{/each}
					</select>
				</div>

				{#if parks(entry?.action)}
					{#each WINDOW_FIELDS as field (field.field)}
						<div class="flex w-full flex-col gap-1 text-sm sm:w-44">
							<label for={`settings-class-${row.id}-${field.field}`}>{field.label}</label>
							<input
								id={`settings-class-${row.id}-${field.field}`}
								type="number"
								value={entry?.[field.field] ?? ''}
								oninput={(event) => setSeconds(row.class, field.field, event.currentTarget.value)}
								aria-describedby={`settings-class-${row.id}-${field.field}-help`}
								class={`${fieldClass} tabular-nums`}
							/>
							<span
								id={`settings-class-${row.id}-${field.field}-help`}
								class="text-xs text-[var(--color-text-muted)]"
							>
								{field.help}
							</span>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	{/each}
</div>
