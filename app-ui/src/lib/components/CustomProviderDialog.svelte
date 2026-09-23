<script lang="ts">
	// The add and edit dialog for one custom provider node (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// One dialog for both, because the fields are the same: a second component would let the add form and
	// the edit form disagree about the prefix rule, which is the one rule the gateway refuses on.
	//
	// The endpoint is stated rather than edited. §7.4 makes `type` and `api_type` part of a node's
	// identity, so changing one is a new node: the edit mode shows the path the gateway will call instead
	// of a control that cannot act (R-26).
	//
	// The joined URL is rendered under the base URL field, live. The gateway appends the path to whatever
	// is typed, so a base URL one segment off is a request to the wrong path, which surfaces as a
	// transport error at call time rather than as a configuration mistake. Showing the joined URL is what
	// makes the field readable before the node exists.
	//
	// Validation runs on submit rather than on every keystroke (§8.4.5), and the messages are the
	// schema's, so the panel and the gateway describe a rule the same way.
	//
	// The copy per type is the reference's own (`AddCompatibleModal.js:7-30`): the vendor URL is the field's
	// value rather than its placeholder, each variant names itself in the name and prefix placeholders, and
	// switching the API type puts the vendor URL back (`:54-62`). The variant's base URL hint is stated under
	// the field whatever the value is, the way the reference's `Input` renders its `hint` prop (`:166`), so
	// the copy that says which URL belongs there is never replaced by the preview; the joined URL is an extra
	// line below it.
	// The reference's **Check** (a key plus an optional model id, validated before the node exists; handler
	// `:93-112`, controls `:168-194`) is deliberately not built: the route it calls
	// has no counterpart in app-serv yet (measured 405; draft 017 F6), and a control that calls a route
	// nothing serves is a dead control (R-26). The draft records it as the one gap this dialog still has.
	import FormIssues from '$lib/components/FormIssues.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { createProviderNode, updateProviderNode } from '$lib/api/provider-nodes';
	import {
		NODE_API_TYPES,
		NODE_API_TYPE_LABELS,
		NODE_TYPE_LABELS,
		nodeEndpointUrl,
		type NodeType,
		type ProviderNode
	} from '$lib/schemas/provider-node';
	import {
		NODE_BASE_URL_DEFAULTS,
		NODE_COPY,
		createProviderNodeBody,
		customProviderDraftFrom,
		customProviderDraftNew,
		schemaCustomProviderDraft,
		updateProviderNodeBody,
		type CustomProviderDraft
	} from '$lib/schemas/provider-node-draft';

	let {
		target,
		type,
		onsaved,
		onclose
	}: {
		/** The node being edited, `'new'` for an add, or null while the dialog is shut. */
		target: ProviderNode | 'new' | null;
		/** The node type an add creates. An edit's type is the node's own. */
		type: NodeType;
		/** Asks the page to re-read the set, so the list shows what the API stored (§8.6.3). */
		onsaved: () => Promise<void>;
		onclose: () => void;
	} = $props();

	const editing = $derived(target !== null && target !== 'new');
	const node = $derived(target === null || target === 'new' ? null : target);

	let draft = $state<CustomProviderDraft>(customProviderDraftNew('openai-compatible'));
	let issues = $state<string[]>([]);
	let error = $state<string | null>(null);
	let saving = $state(false);

	// The copy the dialog states for the type it is opening: the reference keeps one table per variant
	// (`AddCompatibleModal.js:6-28`), so the name and prefix hints cannot describe the two differently.
	const copy = $derived(NODE_COPY[type]);

	// Follows the node the page opened, so a second Edit opens that node rather than the first one's
	// draft, and a cancel leaves nothing behind for the next open.
	$effect(() => {
		draft =
			target === null || target === 'new'
				? customProviderDraftNew(type)
				: customProviderDraftFrom(target);
		issues = [];
		error = null;
	});

	// Switching the wire format puts the vendor URL back in the field: the base URL a chat node was typed
	// against is not the one a responses node should call, and the reference resets it the same way
	// (`AddCompatibleModal.js:45-47`). Only an add resets, because an edit's URL is the stored one.
	function changeAPIType(value: string): void {
		draft.api_type = value === 'responses' ? 'responses' : 'chat';
		if (!editing) draft.base_url = NODE_BASE_URL_DEFAULTS[type];
	}

	// The request the gateway will make, so the base URL is read as a path and not as a host.
	const joined = $derived(
		draft.base_url === ''
			? null
			: nodeEndpointUrl({ type, api_type: draft.api_type, base_url: draft.base_url })
	);

	async function submit(): Promise<void> {
		const parsed = schemaCustomProviderDraft.safeParse(draft);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			error = null;
			return;
		}

		issues = [];
		saving = true;
		const result = node
			? await updateProviderNode(node.id, updateProviderNodeBody(parsed.data))
			: await createProviderNode(createProviderNodeBody(type, parsed.data));
		saving = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		await onsaved();
		onclose();
	}
</script>

<Modal
	title={`${editing ? 'Edit' : 'Add'} ${NODE_TYPE_LABELS[type]}`}
	open={target !== null}
	{onclose}
>
	<div class="flex flex-col gap-3">
		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Name</span>
			<input
				bind:value={draft.name}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder={copy.namePlaceholder}
			/>
		</label>

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Prefix</span>
			<input
				bind:value={draft.prefix}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder={copy.prefixPlaceholder}
			/>
		</label>
		<p class="text-[var(--color-text-muted)]">
			A model string is <span class="font-medium">{draft.prefix || 'prefix'}/model</span>, so the
			prefix is how this node's models are addressed. Letters, digits, dots, dashes, and underscores
			only, and it cannot collide with a provider the registry already ships.
		</p>

		{#if type === 'openai-compatible' && !editing}
			<label class="flex flex-col gap-1">
				<span class="text-[var(--color-text-muted)]">API type</span>
				<select
					value={draft.api_type}
					onchange={(event) => changeAPIType(event.currentTarget.value)}
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				>
					{#each NODE_API_TYPES as option (option)}
						<option value={option}>{NODE_API_TYPE_LABELS[option]}</option>
					{/each}
				</select>
			</label>
		{/if}

		<label class="flex flex-col gap-1">
			<span class="text-[var(--color-text-muted)]">Base URL</span>
			<input
				bind:value={draft.base_url}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				placeholder={NODE_BASE_URL_DEFAULTS[type]}
			/>
		</label>
		<p class="text-[var(--color-text-muted)]">{copy.baseUrlHint}</p>

		{#if joined}
			<p class="text-[var(--color-text-muted)]">
				The gateway will call <span class="font-medium break-all">{joined}</span>.
			</p>
		{/if}

		{#if editing && node}
			<p class="text-[var(--color-text-muted)]">
				The API type and the endpoint path are part of this node's identity, so they are not
				editable. Add a node of the other type instead.
			</p>
		{/if}

		<FormIssues {issues} />

		{#if error}
			<p class="text-[var(--color-danger)]" role="alert">
				The provider was not {editing ? 'saved' : 'added'}. {error}
			</p>
		{/if}
	</div>

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={onclose}>Cancel</button>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
			disabled={saving || draft.name.trim() === '' || draft.prefix.trim() === ''}
			onclick={() => void submit()}
		>
			{saving ? 'Saving' : editing ? 'Save the provider' : 'Add the provider'}
		</button>
	{/snippet}
</Modal>
