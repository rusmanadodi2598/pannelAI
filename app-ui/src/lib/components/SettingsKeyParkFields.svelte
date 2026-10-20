<script lang="ts">
	// The key-park group inside the Routing tab (docs/SPEC-API/001-SPEC-API.md §7.14).
	//
	// Four windows of one shape that differ only in the failure class they park after, so they are data
	// rather than four copies of the same markup, and six blocks of label, input and help line is more
	// markup than the tab has line budget for (SPEC-UI §10, 250). That is the same reason the class editor
	// next door is a component.
	//
	// No native `min` on the inputs, for the reason the tab gives for its counts: a browser bound would
	// refuse the value before Zod sees it, and the message would be the browser's and in its language.
	import type { RoutingSettingsForm } from '$lib/schemas/settings';

	type Props = {
		/** The park windows as the routing tab holds them. Writes land in the tab's draft. */
		park: RoutingSettingsForm['key_park'];
	};

	let { park = $bindable() }: Props = $props();

	const WINDOWS: ReadonlyArray<{
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

	// A park is blank by design when it inherits, and a number input hands back an empty string, which a
	// coerced field would turn into 0 and refuse. Blank is stored as `undefined`, which is what lets the
	// tab send nothing for that window and the gateway apply its own default.
	function setSeconds(field: keyof RoutingSettingsForm['key_park'], raw: string): void {
		const trimmed = raw.trim();
		park = { ...park, [field]: trimmed === '' ? undefined : Number(trimmed) };
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<h3 class="pt-1 text-sm font-semibold">Key park</h3>
<p class="text-sm text-[var(--color-text-muted)]">
	How long a credential is set aside after each kind of failure. A parked credential reopens on its
	own when its window passes; blank means the gateway default applies.
</p>

{#each WINDOWS as window (window.field)}
	<div class="flex flex-col gap-1 text-sm">
		<label for={window.id}>{window.label}</label>
		<input
			id={window.id}
			type="number"
			value={park[window.field] ?? ''}
			oninput={(event) => setSeconds(window.field, event.currentTarget.value)}
			aria-describedby={`${window.id}-help`}
			class={fieldClass}
		/>
		<span id={`${window.id}-help`} class="text-xs text-[var(--color-text-muted)]">
			{window.help}
		</span>
	</div>
{/each}
