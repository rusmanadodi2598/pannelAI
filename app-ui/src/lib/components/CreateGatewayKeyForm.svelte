<script lang="ts">
	// Create-gateway-key form.
	//
	// The screen owns the list and the one-time key modal; this component owns one field and one call,
	// so the screen file stays within the project's line limit and each piece has one job.
	import { createGatewayKey } from '$lib/api/gateway-keys';
	import { schemaCreateGatewayKeyForm, type CreatedGatewayKey } from '$lib/schemas/gateway-key';

	type Props = {
		oncreated: (created: CreatedGatewayKey) => void;
	};

	let { oncreated }: Props = $props();

	let name = $state('');
	let error = $state<string | null>(null);
	let working = $state(false);

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		error = null;

		const parsed = schemaCreateGatewayKeyForm.safeParse({ name });
		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'Check the name.';
			return;
		}

		working = true;
		const result = await createGatewayKey(parsed.data);
		working = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		name = '';
		oncreated(result.data);
	}
</script>

<form class="flex flex-wrap items-end gap-3" onsubmit={submit} novalidate>
	<label class="flex flex-col gap-1.5 text-sm">
		<span class="font-medium">Key name</span>
		<input
			bind:value={name}
			name="name"
			placeholder="Laptop, CI runner"
			aria-invalid={error ? 'true' : undefined}
			class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
		/>
	</label>

	<button
		type="submit"
		disabled={working}
		class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-60"
	>
		{working ? 'Creating' : 'Create gateway key'}
	</button>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]" role="alert">{error}</p>
	{/if}
</form>
