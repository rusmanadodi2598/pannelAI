<script lang="ts">
	// The Vision Adapter tab of /combos (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 2).
	//
	// The screen states its own scope, because the reference shipped four adapters and this port ships one.
	// §6.4 requires the pdf, audio-input, and video-input adapters to be absent rather than disabled, so
	// there is no control for them and no upgrade hint: the note is text.
	//
	// The picker offers the vision models of the providers that are configured right now, which is the
	// reference's own rule for this adapter: it hands the same modal its connected providers
	// (`ModelSelectModal.js:216-219`) and the `vision` capability (`combos/page.js:826-835`), and the
	// capability filter drops a provider that answered with no such model (`:448-451`). A model that is
	// already selected but no longer appears in that list is kept and shown, because dropping it silently
	// would turn a capability change on the provider's side into a configuration the operator never chose.
	import { untrack } from 'svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import ModelPickerDialog from '$lib/components/ModelPickerDialog.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import VisionModelPicker from '$lib/components/VisionModelPicker.svelte';
	import { loadPickerSources } from '$lib/model-picker-data';
	import { pickerSections } from '$lib/schemas/model-picker';
	import type { CatalogModel } from '$lib/schemas/model';
	import type { Provider } from '$lib/schemas/provider';
	import { getVisionAdapter, replaceVisionAdapter } from '$lib/api/vision-adapter';
	import {
		buildVisionAdapterBody,
		schemaVisionAdapterForm,
		visionAdapterStatusText,
		visionAdapterToForm,
		visionAdapterWarning,
		type VisionAdapter,
		type VisionAdapterForm
	} from '$lib/schemas/vision-adapter';

	let adapter = $state<VisionAdapter | null>(null);
	let form = $state<VisionAdapterForm>({ enabled: false, roundRobin: false, models: [] });
	let catalog = $state<CatalogModel[]>([]);
	let providers = $state<Provider[]>([]);
	let pickerFailed = $state(false);
	// True while the capability-filtered read is in flight, so the dialog never states "no provider
	// reports a vision model" over an answer that has not arrived (R-27).
	let pickerLoading = $state(true);
	let pickerOpen = $state(false);

	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let saving = $state(false);
	let saveError = $state<string | null>(null);
	let saved = $state(false);
	let issues = $state<string[]>([]);

	const warning = $derived(visionAdapterWarning(form));
	const status = $derived(visionAdapterStatusText(form));

	// No placeholders and no combos: this picker was read for one capability, and the reference's own
	// capability-filtered picker hides the combos (`ModelSelectModal.js:427`).
	const sections = $derived(
		pickerSections({ catalog, providers, combos: [], placeholders: false })
	);

	$effect(() => {
		untrack(() => {
			void load();
			void loadPicker();
		});
	});

	async function load(): Promise<void> {
		loading = true;
		const result = await getVisionAdapter();
		loading = false;

		if (!result.ok) {
			loadError = result.error.message;
			return;
		}

		loadError = null;
		adapter = result.data;
		form = visionAdapterToForm(result.data);
	}

	async function loadPicker(): Promise<void> {
		const sources = await loadPickerSources({ capability: 'vision' });
		catalog = sources.catalog;
		providers = sources.providers;
		pickerFailed = sources.failed;
		pickerLoading = false;
	}

	function toggleModel(ref: string): void {
		saved = false;
		form.models = form.models.includes(ref)
			? form.models.filter((entry) => entry !== ref)
			: [...form.models, ref];
	}

	async function save(): Promise<void> {
		const parsed = schemaVisionAdapterForm.safeParse(form);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			saveError = null;
			return;
		}

		issues = [];
		saving = true;
		saved = false;
		const result = await replaceVisionAdapter(buildVisionAdapterBody(parsed.data));
		saving = false;

		if (!result.ok) {
			saveError = result.error.message;
			return;
		}

		saveError = null;
		saved = true;
		adapter = result.data;
		form = visionAdapterToForm(result.data);
	}
</script>

<div class="flex flex-col gap-5">
	<p
		class="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 text-sm"
	>
		<span class="font-medium">This adapter covers vision only.</span>
		The pdf, audio-input, and video-input adapters from the reference gateway are not part of this build,
		so this screen has no controls for them. When a request carries an image and the resolved model cannot
		read it, the router prepends one of the models below and the response identity stays the model the
		client asked for.
	</p>

	{#if loading}
		<StateMessage kind="loading" title="Loading the vision adapter" />
	{:else if loadError}
		<StateMessage
			kind="error"
			title="The vision adapter could not be loaded"
			description={loadError}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else}
		<p class="text-sm">
			<span class="text-[var(--color-text-muted)]">Current state:</span>
			{status}
			{#if adapter?.updated_at}
				<span class="text-[var(--color-text-muted)]">, last saved {adapter.updated_at}</span>
			{/if}
		</p>

		{#if warning}
			<p
				class="rounded-[var(--radius-md)] border border-[var(--color-warn)] px-3 py-2 text-sm"
				role="status"
			>
				{warning}
			</p>
		{/if}

		<form
			class="flex flex-col gap-4"
			onsubmit={(event) => {
				event.preventDefault();
				void save();
			}}
		>
			<div class="flex flex-col gap-2">
				<label class="flex min-h-11 items-center gap-2 text-sm">
					<input
						type="checkbox"
						bind:checked={form.enabled}
						class="size-5"
						onchange={() => (saved = false)}
					/>
					<span>Adapt requests that carry images</span>
				</label>

				<label class="flex min-h-11 items-center gap-2 text-sm">
					<input
						type="checkbox"
						bind:checked={form.roundRobin}
						class="size-5"
						onchange={() => (saved = false)}
					/>
					<span>Rotate across the selected models instead of trying them in order</span>
				</label>
			</div>

			<VisionModelPicker
				{form}
				{sections}
				{pickerFailed}
				ontoggle={toggleModel}
				onchoose={() => (pickerOpen = true)}
			/>

			<FormIssues {issues} />

			{#if saveError}
				<p
					class="rounded-[var(--radius-sm)] border border-[var(--color-danger)] px-3 py-2 text-sm"
					role="alert"
				>
					{saveError}
				</p>
			{/if}

			<div class="flex flex-wrap items-center gap-3">
				<button
					type="submit"
					disabled={saving}
					class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)] disabled:opacity-50"
				>
					{saving ? 'Saving' : 'Save the adapter'}
				</button>
				{#if saved}
					<span class="text-sm text-[var(--color-ok)]" role="status">Saved.</span>
				{/if}
			</div>
		</form>

		<!-- Outside the form on purpose: Enter in the dialog's search field must not submit the adapter. -->
		<ModelPickerDialog
			title="Choose vision models"
			open={pickerOpen}
			{sections}
			selected={form.models}
			loading={pickerLoading}
			failed={pickerFailed}
			emptyText="No connected provider reports a vision-capable model right now. Connect one on the Providers screen, or declare the capability on a model you added there."
			ontoggle={toggleModel}
			onclose={() => (pickerOpen = false)}
		/>
	{/if}
</div>
