<script lang="ts">
	// Create-gateway-key form.
	//
	// The screen owns the list and the one-time key modal; this component owns one field and one call,
	// so the screen file stays within the project's line limit and each piece has one job.
	import { createGatewayKey } from '$lib/api/gateway-keys';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import { schemaCreateGatewayKeyForm, type CreatedGatewayKey } from '$lib/schemas/gateway-key';

	type Props = {
		oncreated: (created: CreatedGatewayKey) => void;
	};

	let { oncreated }: Props = $props();

	let name = $state('');
	let error = $state<string | ApiError | null>(null);
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
			error = result.error;
			return;
		}

		name = '';
		oncreated(result.data);
	}
</script>

<!-- Mobile: the field and the action stack at full width, so the three controls on this screen (the name
     field, Create, Refresh) are one column of equal-width rows. On a phone the 256px field kept its width
     and the Create button sat on its own line at a different width (antislop-layoutmobile, Form Row). -->
<form
	class="flex flex-col items-stretch gap-3 sm:flex-row sm:flex-wrap sm:items-end"
	onsubmit={submit}
	novalidate
>
	<label class="flex flex-col gap-1.5 text-sm sm:w-64">
		<span class="font-medium">Key name</span>
		<input
			bind:value={name}
			name="name"
			placeholder="Laptop, CI runner"
			aria-invalid={error ? 'true' : undefined}
			class="min-h-11 w-full rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
		/>
	</label>

	<Button type="submit" disabled={working} aria-busy={working} class="w-full sm:w-auto">
		{working ? 'Creating' : 'Create gateway key'}
	</Button>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]" role="alert"><ApiErrorText {error} /></p>
	{/if}
</form>
