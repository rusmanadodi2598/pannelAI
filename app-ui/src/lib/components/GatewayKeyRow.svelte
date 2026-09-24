<script lang="ts">
	// One row of the gateway keys table.
	//
	// Rename and enable/disable live here because they act on a single key; revocation does not,
	// because it needs a confirmation dialog that belongs to the screen.
	//
	// Every action is icon-only (owner directive, 2026-09-24): the glyph comes from the one icon map and
	// the button carries the action's name as its accessible name and its title, so the control reads the
	// same to a screen reader as the text button it replaced. The edit state is part of the same cell, so
	// its Save and Cancel are icons too.
	import { updateGatewayKey } from '$lib/api/gateway-keys';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import {
		KEY_STATUS_ACTIVE,
		KEY_STATUS_DISABLED,
		type GatewayKey
	} from '$lib/schemas/gateway-key';

	type Props = {
		entry: GatewayKey;
		busy: boolean;
		onrevoke: (entry: GatewayKey) => void;
		onchanged: () => void;
	};

	let { entry, busy, onrevoke, onchanged }: Props = $props();

	const RenameIcon = ROW_ACTION_ICONS.rename.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const RevokeIcon = ROW_ACTION_ICONS.revoke.icon;
	const SaveIcon = ROW_ACTION_ICONS.save.icon;
	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;

	// One target size and one hover wash for every action, so the row reads as a set rather than as
	// three differently styled controls.
	const actionClass =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)] disabled:opacity-50';

	let renaming = $state(false);
	let draft = $state('');
	let localBusy = $state(false);
	let failure = $state<string | null>(null);

	const active = $derived(entry.status === KEY_STATUS_ACTIVE);

	function startRename(): void {
		failure = null;
		renaming = true;
		draft = entry.name;
	}

	async function save(): Promise<void> {
		if (draft.trim().length === 0) {
			renaming = false;
			return;
		}

		localBusy = true;
		const result = await updateGatewayKey(entry.id, { name: draft });
		localBusy = false;

		if (!result.ok) {
			// The field stays open with the operator's draft in it: a rename the gateway refused must not
			// look like one that happened, and closing the field would throw the draft away.
			failure = result.error.message;
			return;
		}

		failure = null;
		renaming = false;
		onchanged();
	}

	async function toggle(): Promise<void> {
		localBusy = true;
		const result = await updateGatewayKey(entry.id, {
			status: active ? KEY_STATUS_DISABLED : KEY_STATUS_ACTIVE
		});
		localBusy = false;

		if (!result.ok) {
			failure = result.error.message;
			return;
		}

		failure = null;
		onchanged();
	}

	const working = $derived(busy || localBusy);
</script>

<tr class="border-t border-[var(--color-border)]">
	<td class="px-3 py-2">
		{#if renaming}
			<input
				bind:value={draft}
				aria-label="New name"
				class="min-h-9 w-48 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2"
			/>
		{:else}
			{entry.name}
		{/if}
	</td>
	<td class="px-3 py-2 font-mono text-xs">{entry.key_hint}</td>
	<td class="px-3 py-2">{entry.status}</td>
	<td class="px-3 py-2">{entry.request_count}</td>
	<td class="px-3 py-2 text-[var(--color-text-muted)]">{entry.last_used_at ?? 'Never'}</td>
	<td class="flex flex-wrap items-center gap-1 px-3 py-2">
		{#if renaming}
			<button
				type="button"
				class={actionClass}
				aria-label="Save"
				title="Save"
				disabled={working}
				onclick={save}><SaveIcon class="size-4" aria-hidden="true" /></button
			>
			<button
				type="button"
				class={actionClass}
				aria-label="Cancel"
				title="Cancel"
				onclick={() => (renaming = false)}><CancelIcon class="size-4" aria-hidden="true" /></button
			>
		{:else}
			<button
				type="button"
				class={actionClass}
				aria-label="Rename"
				title="Rename"
				disabled={working}
				onclick={startRename}><RenameIcon class="size-4" aria-hidden="true" /></button
			>
			<button
				type="button"
				class={actionClass}
				aria-label={active ? 'Disable' : 'Enable'}
				title={active ? 'Disable' : 'Enable'}
				disabled={working}
				onclick={toggle}
			>
				{#if active}
					<DisableIcon class="size-4" aria-hidden="true" />
				{:else}
					<EnableIcon class="size-4" aria-hidden="true" />
				{/if}
			</button>
			<button
				type="button"
				class="{actionClass} text-[var(--color-danger)] hover:text-[var(--color-danger)]"
				aria-label="Revoke"
				title="Revoke"
				onclick={() => onrevoke(entry)}><RevokeIcon class="size-4" aria-hidden="true" /></button
			>
		{/if}

		{#if failure}
			<p role="alert" class="basis-full text-sm text-[var(--color-danger)]">{failure}</p>
		{/if}
	</td>
</tr>
