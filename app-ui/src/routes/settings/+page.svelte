<script lang="ts">
	// Settings, Security tab (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	//
	// Turning login off exposes the panel on the network, so it asks for confirmation and states what
	// changes. Routing, network, and logging groups are U1 work and are not rendered as dead controls.
	import Modal from '$lib/components/Modal.svelte';
	import PasswordChangeForm from '$lib/components/PasswordChangeForm.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { fetchSettings, patchSecuritySettings } from '$lib/api/settings';
	import { schemaSecuritySettingsForm } from '$lib/schemas/settings';
	import { onMount } from 'svelte';

	let requireLogin = $state(true);
	let requireApiKey = $state(true);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let message = $state<string | null>(null);
	let saving = $state(false);
	let confirmingOpenPanel = $state(false);

	onMount(load);

	async function load(): Promise<void> {
		loading = true;
		const result = await fetchSettings();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		requireLogin = result.data.security.require_login;
		requireApiKey = result.data.security.require_api_key;
	}

	async function save(): Promise<void> {
		message = null;
		const parsed = schemaSecuritySettingsForm.safeParse({
			require_login: requireLogin,
			require_api_key: requireApiKey
		});

		if (!parsed.success) {
			message = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchSecuritySettings(parsed.data);
		saving = false;

		if (!result.ok) {
			message = result.error.message;
			await load();
			return;
		}

		requireLogin = result.data.security.require_login;
		requireApiKey = result.data.security.require_api_key;
		message = requireLogin ? 'Saved.' : 'Saved. The panel no longer asks for a password.';
	}
</script>

<section class="flex max-w-2xl flex-col gap-6">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			Security today. Routing, network, and logging groups arrive in the next phase.
		</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading settings" />
	{:else if error}
		<StateMessage kind="error" title="Settings could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else}
		<div
			class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
		>
			<h2 class="text-sm font-semibold">Access</h2>

			<label class="flex items-start gap-3 text-sm">
				<input
					type="checkbox"
					checked={requireLogin}
					onchange={(event) => {
						if (event.currentTarget.checked) {
							requireLogin = true;
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
				<input type="checkbox" bind:checked={requireApiKey} />
				<span>
					Require a gateway key on data plane requests
					<span class="block text-[var(--color-text-muted)]">
						Applies to /chat/completions and the other client-facing routes.
					</span>
				</span>
			</label>

			<div class="flex items-center gap-3">
				<button
					type="button"
					disabled={saving}
					class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
					onclick={save}>Save access settings</button
				>
				{#if message}
					<p class="text-sm text-[var(--color-text-muted)]" role="status">{message}</p>
				{/if}
			</div>
		</div>

		<PasswordChangeForm />
	{/if}
</section>

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
				requireLogin = false;
				void save();
			}}>Turn it off</button
		>
	{/snippet}
</Modal>
