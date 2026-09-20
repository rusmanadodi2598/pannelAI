<script lang="ts">
	// The add and edit dialog for one proxy candidate (docs/SPEC-UI/001-SPEC-UI.md §6.9).
	//
	// One dialog for both, because the fields are the same and only the password hint differs. The
	// password is write-only, so an edit starts with the field empty and the hint says what that means:
	// empty keeps the stored secret, a value replaces it. That is also why the panel cannot show the
	// current one, and why the field is never prefilled.
	//
	// The candidate test is offered here, before a save, because §6.9 asks for a candidate test: an
	// address that cannot carry a request should be found out before it joins the pool. A failed probe
	// renders as a result with its reason, not as a form error, because the probe answered the question
	// that was asked. The test route stores nothing, so a candidate can be probed before it has a name.
	//
	// Validation runs on submit rather than on every keystroke (§8.4.5), and the messages are the
	// schema's, so the panel and the API describe a rule the same way.
	import { untrack } from 'svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import ProxyFormFields from '$lib/components/ProxyFormFields.svelte';
	import { createProxy, testProxyCandidate, updateProxy } from '$lib/api/proxies';
	import {
		proxyTestStateLabel,
		schemaProxyCandidate,
		schemaProxyForm,
		type Proxy,
		type ProxyTest
	} from '$lib/schemas/proxy';
	import {
		proxyCandidateFromDraft,
		proxyDraftFrom,
		type ProxyFormDraft
	} from '$lib/schemas/proxy-form';
	import { formatTimestamp } from '$lib/utils/time';

	let {
		target,
		onsaved,
		onclose
	}: {
		/** The candidate being edited, `'new'` for an add, or null while the dialog is shut. */
		target: Proxy | 'new' | null;
		/** Asks the page to re-read the pool, so the table shows what the API stored (§8.6.3). */
		onsaved: () => Promise<void>;
		onclose: () => void;
	} = $props();

	let draft = $state<ProxyFormDraft>(untrack(() => proxyDraftFrom(target)));
	let issues = $state<string[]>([]);
	let saving = $state(false);
	let testing = $state(false);
	let result = $state<ProxyTest | null>(null);

	// Follows the row the page opened, so a second Edit opens that row rather than the first one's
	// draft, and a cancel leaves nothing behind for the next open.
	$effect(() => {
		draft = proxyDraftFrom(target);
		issues = [];
		result = null;
	});

	const editing = $derived(target !== null && target !== 'new');

	async function test(): Promise<void> {
		issues = [];
		result = null;

		const parsed = schemaProxyCandidate.safeParse(proxyCandidateFromDraft(draft));
		if (!parsed.success) {
			issues = [parsed.error.issues[0]?.message ?? 'Check these values.'];
			return;
		}

		testing = true;
		const answer = await testProxyCandidate(parsed.data);
		testing = false;

		if (!answer.ok) {
			issues = [answer.error.message];
			return;
		}

		result = answer.data;
	}

	async function save(): Promise<void> {
		issues = [];
		result = null;

		const parsed = schemaProxyForm.safeParse(draft);
		if (!parsed.success) {
			issues = [parsed.error.issues[0]?.message ?? 'Check these values.'];
			return;
		}

		saving = true;
		const answer =
			target !== null && target !== 'new'
				? await updateProxy(target.id, parsed.data)
				: await createProxy(parsed.data);
		saving = false;

		if (!answer.ok) {
			issues = [answer.error.message];
			return;
		}

		await onsaved();
		onclose();
	}

	const hintClass = 'text-xs text-[var(--color-text-muted)]';
</script>

<Modal title={editing ? 'Edit this proxy' : 'Add a proxy'} open={target !== null} {onclose}>
	<div class="flex flex-col gap-4">
		<FormIssues {issues} />

		<ProxyFormFields bind:value={draft} {target} />

		<div class="flex flex-wrap items-center gap-3 border-t border-[var(--color-border)] pt-4">
			<button
				type="button"
				disabled={testing}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-60"
				onclick={() => void test()}
			>
				{testing ? 'Testing' : 'Test this candidate'}
			</button>

			{#if result}
				<p role="status" class="text-sm">
					{proxyTestStateLabel(result.state)} in {result.latency_ms}ms, checked
					{formatTimestamp(result.checked_at)}.
					{#if result.message}
						<span class="block {hintClass}">{result.message}</span>
					{/if}
				</p>
			{/if}
		</div>
	</div>

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={onclose}>Cancel</button>
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={() => void save()}
		>
			{saving ? 'Saving' : editing ? 'Save proxy' : 'Add proxy'}
		</button>
	{/snippet}
</Modal>
