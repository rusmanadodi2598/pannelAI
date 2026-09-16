<script lang="ts">
	// One row of the gateway keys table.
	//
	// Rename and enable/disable live here because they act on a single key; revocation does not,
	// because it needs a confirmation dialog that belongs to the screen.
	import { updateGatewayKey } from '$lib/api/gateway-keys';
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

	let renaming = $state(false);
	let draft = $state('');
	let localBusy = $state(false);

	const active = $derived(entry.status === KEY_STATUS_ACTIVE);

	function startRename(): void {
		renaming = true;
		draft = entry.name;
	}

	async function save(): Promise<void> {
		if (draft.trim().length === 0) {
			renaming = false;
			return;
		}

		localBusy = true;
		await updateGatewayKey(entry.id, { name: draft });
		localBusy = false;
		renaming = false;
		onchanged();
	}

	async function toggle(): Promise<void> {
		localBusy = true;
		await updateGatewayKey(entry.id, {
			status: active ? KEY_STATUS_DISABLED : KEY_STATUS_ACTIVE
		});
		localBusy = false;
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
	<td class="flex flex-wrap gap-2 px-3 py-2">
		{#if renaming}
			<button type="button" class="underline" disabled={working} onclick={save}>Save</button>
			<button type="button" class="underline" onclick={() => (renaming = false)}>Cancel</button>
		{:else}
			<button type="button" class="underline" disabled={working} onclick={startRename}
				>Rename</button
			>
			<button type="button" class="underline" disabled={working} onclick={toggle}>
				{active ? 'Disable' : 'Enable'}
			</button>
			<button
				type="button"
				class="underline text-[var(--color-danger)]"
				onclick={() => onrevoke(entry)}>Revoke</button
			>
		{/if}
	</td>
</tr>
