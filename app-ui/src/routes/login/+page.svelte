<script lang="ts">
	// Password login (docs/SPEC-UI/001-SPEC-UI.md §6.1).
	//
	// One field, no username: the API authenticates with a single password and a Redis-backed lockout.
	// The lockout message states the rule rather than a vague failure, because an operator who is
	// locked out needs to know how long the wait is.
	import { session } from '$lib/stores/session.svelte';
	import { schemaLoginForm } from '$lib/schemas/auth';

	let password = $state('');
	let submitting = $state(false);
	let message = $state<string | null>(null);
	let fieldError = $state<string | null>(null);

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		fieldError = null;
		message = null;

		const parsed = schemaLoginForm.safeParse({ password });
		if (!parsed.success) {
			fieldError = parsed.error.issues[0]?.message ?? 'Check the password.';
			return;
		}

		submitting = true;
		const failure = await session.signIn(parsed.data.password);
		submitting = false;

		if (failure) {
			password = '';
			message = failure;
		}
	}
</script>

<div class="mx-auto flex min-h-screen w-full max-w-sm flex-col justify-center gap-6 p-6">
	<div class="flex flex-col gap-1">
		<p class="text-xs font-medium text-[var(--color-text-muted)]">KENTANG TECH</p>
		<h1 class="text-xl font-semibold tracking-tight">Sign in to pannelAI</h1>
	</div>

	<form class="flex flex-col gap-3" onsubmit={submit} novalidate>
		<label class="flex flex-col gap-1.5 text-sm">
			<span class="font-medium">Panel password</span>
			<input
				type="password"
				name="password"
				bind:value={password}
				autocomplete="current-password"
				aria-invalid={fieldError ? 'true' : undefined}
				aria-describedby={fieldError ? 'password-error' : undefined}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3"
			/>
		</label>

		{#if fieldError}
			<p id="password-error" class="text-sm text-[var(--color-danger)]" role="alert">
				{fieldError}
			</p>
		{/if}

		{#if message}
			<p class="text-sm text-[var(--color-danger)]" role="alert">{message}</p>
		{/if}

		<button
			type="submit"
			disabled={submitting}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-60"
		>
			{submitting ? 'Signing in' : 'Sign in'}
		</button>
	</form>

	{#if session.status?.password_configured === false}
		<p class="text-sm text-[var(--color-text-muted)]">
			No password is configured yet. Set one in app-serv, then sign in here.
		</p>
	{/if}
</div>
