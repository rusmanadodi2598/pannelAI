<script lang="ts">
	// Create one upstream endpoint (docs/SPEC-UI/001-SPEC-UI.md §6.3, Connections).
	//
	// The provider is fixed rather than chosen here: the form renders inside one provider's own screen, and
	// offering a picker would mean loading the registry into a select, which §6.3's pagination discipline
	// rules out.
	//
	// The credential is required for the key auth types and optional for the rest. §7.5 refuses an
	// `api_key` endpoint with no key (`service/endpoint_create.go`, `buildEndpoint`) because an account that
	// can never route is worse than a refused request, so the panel states that rule rather than letting the
	// round trip report it. For `oauth` and the no-auth spellings the endpoint is created empty and given
	// keys from the detail drawer, which is the same code path either way.
	//
	// A service-account endpoint is the one shape that needs more than a key: its project and location are
	// what its calls address, so choosing that auth type reveals the pair beside the JSON key the endpoint
	// mints its bearer from. The pair is offered only there, because the API refuses it on every other type.
	import { createEndpoint } from '$lib/api/endpoints';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { CONTROL_ICONS } from '$lib/icons';
	import { AUTH_TYPES, AUTH_TYPE_LABELS } from '$lib/schemas/endpoint';
	import {
		REQUIRES_KEY_AUTH_TYPES,
		SERVICE_ACCOUNT_AUTH_TYPE
	} from '$lib/schemas/endpoint-credential';
	import { schemaCreateEndpointForm } from '$lib/schemas/endpoint-write';

	let { providerId, oncreated }: { providerId: string; oncreated: () => void } = $props();

	const AddIcon = CONTROL_ICONS.add.icon;

	let label = $state('');
	let authType = $state<string>('api_key');
	let priority = $state('');
	let keyValue = $state('');
	let projectId = $state('');
	let location = $state('');
	let saving = $state(false);
	let error = $state<string | ApiError | null>(null);

	// The field's name follows the rule it carries: for a key auth type it is the credential the endpoint
	// cannot be created without, and for the rest it is a first key that may be left blank. A
	// service-account credential is the JSON key file, and the field says so.
	const keyRequired = $derived(REQUIRES_KEY_AUTH_TYPES.has(authType));
	const serviceAccount = $derived(authType === SERVICE_ACCOUNT_AUTH_TYPE);
	const keyLabel = $derived(
		keyRequired ? (serviceAccount ? 'Service Account JSON' : 'Key') : 'First key (optional)'
	);

	async function submit(): Promise<void> {
		const parsed = schemaCreateEndpointForm.safeParse({
			provider_id: providerId,
			label,
			auth_type: authType,
			priority: priority === '' ? undefined : priority,
			keys: keyValue === '' ? [] : [{ value: keyValue }],
			// The pair travels only with the auth type that reads it, so values typed before the select
			// changed back cannot ride along.
			sa_project_id: serviceAccount ? projectId : undefined,
			sa_location: serviceAccount ? location : undefined
		});

		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'That endpoint cannot be created.';
			return;
		}

		saving = true;
		const result = await createEndpoint(parsed.data);
		saving = false;

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		label = '';
		priority = '';
		keyValue = '';
		projectId = '';
		location = '';
		oncreated();
	}
</script>

<div class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3">
	<span class="text-sm font-medium">Add an endpoint for {providerId}</span>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]"><ApiErrorText {error} /></p>
	{/if}

	<div class="flex flex-wrap items-end gap-3 text-sm">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Label</span>
			<input
				bind:value={label}
				placeholder="Primary"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Auth type</span>
			<select
				bind:value={authType}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			>
				{#each AUTH_TYPES as type (type)}
					<option value={type}>{AUTH_TYPE_LABELS[type] ?? type}</option>
				{/each}
			</select>
		</label>

		{#if serviceAccount}
			<label class="flex flex-col gap-1">
				<span class="text-[var(--color-text-muted)]">Project ID</span>
				<input
					bind:value={projectId}
					placeholder="my-project"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				/>
			</label>

			<label class="flex flex-col gap-1">
				<span class="text-[var(--color-text-muted)]">Location</span>
				<input
					bind:value={location}
					placeholder="us-central1"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				/>
			</label>
		{/if}

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Priority (optional)</span>
			<input
				bind:value={priority}
				inputmode="numeric"
				placeholder="1"
				class="min-h-11 w-28 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 tabular-nums"
			/>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">{keyLabel}</span>
			<input
				bind:value={keyValue}
				type="password"
				autocomplete="off"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			/>
		</label>

		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 font-medium text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={saving}
			onclick={submit}
		>
			<AddIcon class="size-4" aria-hidden="true" />
			Add endpoint
		</button>
	</div>
</div>
