<script lang="ts">
	// Settings, Network tab (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	// PATCH /api/v1/settings, the network group.
	//
	// The outbound proxy settings are global: they decide the path every upstream call takes, not one
	// endpoint's. Per-endpoint binding is not part of v1, so this tab says that rather than leaving an
	// operator hunting for a control that does not exist.
	//
	// §6.13 asks this tab to link to /proxy-pools instead of duplicating its form. That screen is U2 and
	// does not exist yet, while §12 puts the Network settings in U1, so U1 renders the fields here and
	// the spec records the U2 conversion. A link to a route the panel does not have would break R-24.
	//
	// `outbound_no_proxy` stays a comma-separated string end to end, because that is what the API sends
	// and accepts; the schema normalizes it (trim, lowercase, dedupe, drop empty) without changing shape.
	import { untrack } from 'svelte';
	import { patchNetworkSettings } from '$lib/api/settings';
	import {
		schemaNetworkSettingsForm,
		settingsGroupDirty,
		type NetworkSettingsForm
	} from '$lib/schemas/settings';

	type Props = {
		loaded: NetworkSettingsForm;
		/** Asks the page to re-read the whole settings document after a successful write (§8.6.3). */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	const initial = untrack(() => ({ ...loaded }));
	let draft = $state<NetworkSettingsForm>({ ...initial });
	let server = $state<NetworkSettingsForm>({ ...initial });

	let message = $state<string | null>(null);
	let saving = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	async function save(): Promise<void> {
		message = null;
		const parsed = schemaNetworkSettingsForm.safeParse(draft);
		if (!parsed.success) {
			message = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchNetworkSettings(parsed.data);
		saving = false;

		if (!result.ok) {
			message = result.error.message;
			return;
		}

		const saved = { ...result.data.network };
		server = saved;
		draft = saved;
		message = 'Saved. The next outbound call uses this path.';
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
	<h2 class="text-sm font-semibold">Network</h2>
	<p class="text-sm text-[var(--color-text-muted)]">
		Global outbound routing. These apply to every upstream call the gateway makes, whichever
		endpoint it is for. Per-endpoint binding is not part of v1.
	</p>

	<label class="flex items-start gap-3 text-sm">
		<input type="checkbox" bind:checked={draft.outbound_proxy_enabled} class="mt-1" />
		<span>
			Send upstream calls through a proxy
			<span class="block text-[var(--color-text-muted)]">
				When this is on, a proxy URL is required. When it is off, the URL is kept but unused.
			</span>
		</span>
	</label>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-outbound-url">Outbound proxy URL</label>
		<input
			id="settings-outbound-url"
			type="text"
			bind:value={draft.outbound_proxy_url}
			placeholder="http://proxy.internal:8080"
			aria-describedby="settings-outbound-url-help"
			class={fieldClass}
		/>
		<span id="settings-outbound-url-help" class="text-xs text-[var(--color-text-muted)]">
			An absolute http or https URL.
		</span>
	</div>

	<div class="flex flex-col gap-1 text-sm">
		<label for="settings-outbound-no-proxy">Bypass the proxy for these hosts</label>
		<input
			id="settings-outbound-no-proxy"
			type="text"
			bind:value={draft.outbound_no_proxy}
			placeholder="localhost,10.0.0.0/8"
			aria-describedby="settings-outbound-no-proxy-help"
			class={fieldClass}
		/>
		<span id="settings-outbound-no-proxy-help" class="text-xs text-[var(--color-text-muted)]">
			Comma-separated. Duplicates and empty entries are removed on save.
		</span>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={save}>Save network settings</button
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
