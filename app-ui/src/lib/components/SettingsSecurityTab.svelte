<script lang="ts">
	// Settings, Security tab (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	// PATCH /api/v1/settings, the security group.
	//
	// Turning login off exposes the panel on the network, so it asks for confirmation and states what
	// changes rather than applying the toggle on the click that flipped it. The password form is part of
	// this tab because a password is a security setting, and giving it a second screen would split one
	// concern across two places.
	import { untrack } from 'svelte';
	import Modal from '$lib/components/Modal.svelte';
	import PasswordChangeForm from '$lib/components/PasswordChangeForm.svelte';
	import { patchSecuritySettings } from '$lib/api/settings';
	import {
		schemaSecuritySettingsForm,
		settingsGroupDirty,
		type SecuritySettingsForm
	} from '$lib/schemas/settings';

	type Props = {
		loaded: SecuritySettingsForm;
		/** Asks the page to re-read the whole settings document after a successful write (§8.6.3). */
		onrefresh: () => Promise<void>;
	};

	let { loaded, onrefresh }: Props = $props();

	const initial = untrack(() => ({ ...loaded }));
	let draft = $state<SecuritySettingsForm>({ ...initial });
	let server = $state<SecuritySettingsForm>({ ...initial });

	let message = $state<string | null>(null);
	let saving = $state(false);
	let confirmingOpenPanel = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	async function save(): Promise<void> {
		message = null;
		const parsed = schemaSecuritySettingsForm.safeParse(draft);
		if (!parsed.success) {
			message = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchSecuritySettings(parsed.data);
		saving = false;

		if (!result.ok) {
			message = result.error.message;
			// A refused write means the draft may not match what the gateway holds, so the draft goes
			// back to the last confirmed value rather than staying as the operator left it.
			draft = { ...server };
			return;
		}

		const saved = { ...result.data.security };
		server = saved;
		draft = saved;
		message = saved.require_login ? 'Saved.' : 'Saved. The panel no longer asks for a password.';
		await onrefresh();
	}

	function discard(): void {
		draft = { ...server };
		message = null;
	}
</script>

<div class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4">
	<h2 class="text-sm font-semibold">Access</h2>

	<label class="flex items-start gap-3 text-sm">
		<input
			type="checkbox"
			checked={draft.require_login}
			class="mt-1"
			onchange={(event) => {
				if (event.currentTarget.checked) {
					draft.require_login = true;
					void save();
				} else {
					confirmingOpenPanel = true;
				}
			}}
		/>
		<span>
			Require a password to open the panel
			<span class="block text-[var(--color-text-muted)]">
				With this off, anyone who can reach the panel origin can manage keys and providers.
			</span>
		</span>
	</label>

	<label class="flex items-start gap-3 text-sm">
		<input type="checkbox" bind:checked={draft.require_api_key} class="mt-1" />
		<span>
			Require a gateway key on data plane requests
			<span class="block text-[var(--color-text-muted)]">
				Applies to /chat/completions and the other client-facing routes.
			</span>
		</span>
	</label>

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={save}>Save access settings</button
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

<PasswordChangeForm />

<Modal
	title="Turn off the password requirement"
	open={confirmingOpenPanel}
	onclose={() => (confirmingOpenPanel = false)}
>
	<p class="text-sm">
		Saving this makes the panel reachable without a password from anywhere that can reach this
		origin. Gateway keys and provider credentials become readable to anyone with network access.
	</p>

	{#snippet footer()}
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={() => (confirmingOpenPanel = false)}>Keep the password</button
		>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-3 text-sm font-medium text-white"
			onclick={() => {
				confirmingOpenPanel = false;
				draft.require_login = false;
				void save();
			}}>Turn it off</button
		>
	{/snippet}
</Modal>
