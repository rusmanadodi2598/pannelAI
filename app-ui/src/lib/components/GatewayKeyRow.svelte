<script lang="ts">
	// One row of the gateway keys table: rename and enable/disable act on one key, so they live here;
	// deletion opens a screen-owned confirmation. Delete is the panel's label for the API's revocation
	// (SPEC-API §7.3). Actions are icon-only, with their names in aria-label and title.
	import { updateGatewayKey } from '$lib/api/gateway-keys';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { formatTimestamp } from '$lib/utils/time';
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

	// Actions take size and state from the shared button; no text-* utilities here, because two would
	// resolve by stylesheet order rather than intent (DESIGN.md §3.5).
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
				class="min-h-11 w-48 max-w-full rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2"
			/>
		{:else}
			{entry.name}
		{/if}
	</td>
	<td data-label="Key" class="px-3 py-2 font-mono text-xs">{entry.key_hint}</td>
	<td data-label="Status" class="px-3 py-2">{entry.status}</td>
	<td data-label="Requests" class="px-3 py-2">{entry.request_count}</td>
	<td data-label="Last used" class="px-3 py-2 text-[var(--color-text-muted)]"
		>{entry.last_used_at ? formatTimestamp(entry.last_used_at) : 'Never'}</td
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
