<script lang="ts">
	// One row of the gateway keys table.
	//
	// Rename and enable/disable live here because they act on a single key; deletion does not, because it
	// needs a confirmation dialog that belongs to the screen. The destructive action is labelled Delete,
	// which is the reference's own name for it (`EndpointPageClient.js`); it calls the terminal
	// DELETE route of SPEC-API §7.3, which the domain calls revocation.
	//
	// Every action is icon-only: the glyph comes from the one icon map and
	// the button carries the action's name as its accessible name and its title, so the control reads the
	// same to a screen reader as the text button it replaced. The edit state is part of the same cell, so
	// its Save and Cancel are icons too.
	import { updateGatewayKey } from '$lib/api/gateway-keys';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import {
		KEY_STATUS_ACTIVE,
		KEY_STATUS_DISABLED,
		type GatewayKey
	} from '$lib/schemas/gateway-key';

	type Props = {
		entry: GatewayKey;
		busy: boolean;
		ondelete: (entry: GatewayKey) => void;
		onchanged: () => void;
	};

	let { entry, busy, ondelete, onchanged }: Props = $props();

	const RenameIcon = ROW_ACTION_ICONS.rename.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;
	const SaveIcon = ROW_ACTION_ICONS.save.icon;
	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;

	// Every action takes its size and its states from the shared button: four ghosts and one quiet danger
	// glyph, so the row reads as a set. No colour is decided here, where two `text-*` utilities would
	// resolve by stylesheet order rather than by intent (DESIGN.md §3.5).
	let renaming = $state(false);
	let draft = $state('');
	let localBusy = $state(false);
	let failure = $state<ApiError | null>(null);

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
			failure = result.error;
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
			failure = result.error;
			return;
		}

		failure = null;
		onchanged();
	}

	const working = $derived(busy || localBusy);
</script>

<tr class="border-t border-[var(--color-border)]">
	<td data-label="Name" class="px-3 py-2">
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
	<td data-label="Key" class="px-3 py-2 font-mono text-xs">{entry.key_hint}</td>
	<td data-label="Status" class="px-3 py-2">{entry.status}</td>
	<td data-label="Requests" class="px-3 py-2">{entry.request_count}</td>
	<td data-label="Last used" class="px-3 py-2 text-[var(--color-text-muted)]"
		>{entry.last_used_at ?? 'Never'}</td
	>
	<td data-label="Actions" class="px-3 py-2">
		<div class="flex flex-wrap items-center gap-1">
			{#if renaming}
				<Button
					variant="ghost"
					size="icon"
					aria-label="Save"
					title="Save"
					disabled={working}
					onclick={save}><SaveIcon class="size-4" aria-hidden="true" /></Button
				>
				<Button
					variant="ghost"
					size="icon"
					aria-label="Cancel"
					title="Cancel"
					onclick={() => (renaming = false)}
					><CancelIcon class="size-4" aria-hidden="true" /></Button
				>
			{:else}
				<Button
					variant="ghost"
					size="icon"
					aria-label="Rename"
					title="Rename"
					disabled={working}
					onclick={startRename}><RenameIcon class="size-4" aria-hidden="true" /></Button
				>
				<Button
					variant="ghost"
					size="icon"
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
				</Button>
				<Button
					variant="destructive-ghost"
					size="icon"
					aria-label="Delete"
					title="Delete"
					onclick={() => ondelete(entry)}><DeleteIcon class="size-4" aria-hidden="true" /></Button
				>
			{/if}

			{#if failure}
				<p role="alert" class="basis-full text-sm text-[var(--color-danger)]">
					<ApiErrorText error={failure} />
				</p>
			{/if}
		</div>
	</td>
</tr>
