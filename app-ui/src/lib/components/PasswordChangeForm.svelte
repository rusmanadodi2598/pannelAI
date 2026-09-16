<script lang="ts">
	// Change password form.
	//
	// It posts to its own endpoint rather than through a settings PATCH, and it never prefills a
	// value, because the API does not return stored secrets and a prefilled field would be a lie.
	import { changePassword } from '$lib/api/auth';
	import { schemaChangePasswordForm } from '$lib/schemas/auth';

	let currentPassword = $state('');
	let newPassword = $state('');
	let message = $state<string | null>(null);
	let error = $state<string | null>(null);
	let working = $state(false);

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = null;
		message = null;

		const parsed = schemaChangePasswordForm.safeParse({
			current_password: currentPassword,
			new_password: newPassword
		});

		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'Check both fields.';
			return;
		}

		working = true;
		const result = await changePassword(parsed.data);
		working = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		currentPassword = '';
		newPassword = '';
		message = 'Password changed.';
	}
</script>

<form
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
	onsubmit={submit}
	novalidate
>
	<h2 class="text-sm font-semibold">Change the panel password</h2>

	<label class="flex flex-col gap-1.5 text-sm">
		<span class="font-medium">Current password</span>
		<input
			type="password"
			bind:value={currentPassword}
			autocomplete="current-password"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
		/>
	</label>

	<label class="flex flex-col gap-1.5 text-sm">
		<span class="font-medium">New password</span>
		<input
			type="password"
			bind:value={newPassword}
			autocomplete="new-password"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
		/>
	</label>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]" role="alert">{error}</p>
	{/if}
	{#if message}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{message}</p>
	{/if}

	<button
		type="submit"
		disabled={working}
		class="min-h-11 self-start rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-60"
	>
		{working ? 'Changing' : 'Change password'}
	</button>
</form>
