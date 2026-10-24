// The selection a bulk bar acts on, and the three writes every bulk route shares (docs/SPEC-UI/001-SPEC-UI.md
// §6.1, §6.2). Two screens run this orchestration on the same endpoints, so it is written once here: the ids
// named by hand or the claim on the whole set, the count echoed back as `confirm_count`, one error slot, and
// the re-read each write is followed by.
//
// The sentence a screen says after a write lands stays at the call site, because the rows have different
// names there and the store has no business choosing one.

import { bulkDelete, bulkStatus, bulkTags, type BulkKind } from '$lib/api/bulk-mutation';
import type { ApiResult } from '$lib/api/client';
import type { ApiError } from '$lib/api/errors';
import type { BulkMutationResult, BulkSelector } from '$lib/schemas/bulk-mutation';
import { emptySelection, toggleAll, toggleRow, type Selection } from '$lib/utils/selection';

type Options = {
	kind: BulkKind;
	/** The ids the list currently holds, which is what a row toggle reconciles an all-selection against. */
	ids: () => string[];
	/** The count the server reports for the whole set, which is what Select-All claims. */
	total: () => number;
	/** Select-All on a scoped list sends the scope with it, and never the ids of the page on screen. */
	scope?: () => Pick<BulkSelector, 'provider_id' | 'auth_type' | 'tag'>;
	/** Re-read the list after a write landed, so the rows show what the server recorded. */
	onapplied: () => Promise<void> | void;
};

export function createBulkActions(options: Options) {
	let selected = $state<Selection>(emptySelection);
	let error = $state<ApiError | null>(null);
	let busy = $state(false);

	const count = $derived(selected.all ? options.total() : selected.ids.length);

	function selector(): BulkSelector {
		return selected.all ? { all: true, ...options.scope?.() } : { ids: selected.ids };
	}

	function toggle(id: string, checked: boolean): void {
		selected = toggleRow(selected, id, checked, options.ids());
	}

	function toggleEvery(checked: boolean): void {
		selected = toggleAll(checked);
	}

	function clear(): void {
		selected = emptySelection;
	}

	async function write(
		call: () => Promise<ApiResult<BulkMutationResult>>
	): Promise<BulkMutationResult | null> {
		if (count === 0) return null;
		busy = true;
		const result = await call();
		busy = false;

		if (!result.ok) {
			error = result.error;
			return null;
		}

		error = null;
		await options.onapplied();
		return result.data;
	}

	function setStatus(status: 'active' | 'disabled'): Promise<BulkMutationResult | null> {
		return write(() => bulkStatus(options.kind, selector(), status, count));
	}

	// The bar hands over a tag it already normalized; the server canonicalizes once more and refuses a
	// label too long to store rather than cutting it.
	function tag(value: string): Promise<BulkMutationResult | null> {
		return write(() => bulkTags(options.kind, selector(), [value], [], false, count));
	}

	// The set is gone once this lands, so the selection goes with it: a bar left counting rows that no
	// longer exist offers a second erase of nothing, and `confirm_count` refuses it.
	async function remove(): Promise<BulkMutationResult | null> {
		const done = await write(() => bulkDelete(options.kind, selector(), count));
		if (done) clear();
		return done;
	}

	return {
		get selected() {
			return selected;
		},
		get error() {
			return error;
		},
		get busy() {
			return busy;
		},
		get count() {
			return count;
		},
		toggle,
		toggleEvery,
		clear,
		setStatus,
		tag,
		remove
	};
}
