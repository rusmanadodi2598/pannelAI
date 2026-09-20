<script lang="ts">
	// The proxy form's fields (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// A bindable group, so the dialog keeps the draft and the state and this file keeps the markup. The
	// hints are linked with `aria-describedby` rather than nested inside the label, because a wrapping
	// label makes the hint part of the field's accessible name: a screen reader would read the rule as
	// the field's name.
	import { PROXY_PROTOCOLS, PROXY_PROTOCOL_LABELS, type Proxy } from '$lib/schemas/proxy';
	import { proxyPasswordHelp, type ProxyFormDraft } from '$lib/schemas/proxy-form';

	let {
		value = $bindable(),
		target
	}: {
		value: ProxyFormDraft;
		/** The row being edited, `'new'` for an add, or null while the dialog is shut. */
		target: Proxy | 'new' | null;
	} = $props();

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
	const hintClass = 'text-xs text-[var(--color-text-muted)]';
</script>

<div class="flex flex-col gap-1 text-sm">
	<label for="proxy-label">Label</label>
	<input id="proxy-label" type="text" bind:value={value.label} class={fieldClass} />
	<span id="proxy-label-help" class={hintClass}>What the row is called in the pool.</span>
</div>

<div class="flex flex-col gap-1 text-sm">
	<label for="proxy-protocol">Protocol</label>
	<select id="proxy-protocol" bind:value={value.protocol} class={fieldClass}>
		{#each PROXY_PROTOCOLS as protocol (protocol)}
			<option value={protocol}>{PROXY_PROTOCOL_LABELS[protocol]}</option>
		{/each}
	</select>
</div>

<div class="flex flex-wrap gap-3">
	<div class="flex min-w-48 flex-1 flex-col gap-1 text-sm">
		<label for="proxy-host">Host</label>
		<input
			id="proxy-host"
			type="text"
			bind:value={value.host}
			aria-describedby="proxy-host-help"
			class={fieldClass}
		/>
		<span id="proxy-host-help" class={hintClass}>
			A name, an IPv4 address, or an IPv6 address in brackets.
		</span>
	</div>

	<div class="flex w-32 flex-col gap-1 text-sm">
		<label for="proxy-port">Port</label>
		<input
			id="proxy-port"
			type="text"
			inputmode="numeric"
			bind:value={value.port}
			aria-describedby="proxy-port-help"
			class={fieldClass}
		/>
		<span id="proxy-port-help" class={hintClass}>1 to 65535.</span>
	</div>
</div>

<div class="flex flex-col gap-1 text-sm">
	<label for="proxy-username">Username</label>
	<input id="proxy-username" type="text" bind:value={value.username} class={fieldClass} />
	<span id="proxy-username-help" class={hintClass}>Optional.</span>
</div>

<div class="flex flex-col gap-1 text-sm">
	<label for="proxy-password">Password</label>
	<input
		id="proxy-password"
		type="password"
		bind:value={value.password}
		aria-describedby="proxy-password-help"
		class={fieldClass}
	/>
	<span id="proxy-password-help" class={hintClass}>{proxyPasswordHelp(target)}</span>
</div>

<label class="flex items-start gap-3 text-sm">
	<input type="checkbox" class="mt-1 size-4" bind:checked={value.enabled} />
	<span>
		Enabled
		<span class="block {hintClass}">
			A disabled candidate stays in the pool with its test result, and is not offered as one to use.
		</span>
	</span>
</label>
