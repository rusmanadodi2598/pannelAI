// The one model a bulk selection follows on every screen that offers one.
//
// A selection is either an id set or a claim on the whole filtered set, never both, because the API
// refuses a selector that names two. That makes Select-All and the row checkboxes two views of one
// state rather than two states: every loaded row belongs to an all-selection, so each checkbox has
// to read as checked, and taking one row out of an all-selection cannot be sent as "all except this
// one" - it has to fall back to the ids the page actually holds, which is a smaller set and reads
// that way in the bar.

export type Selection = {
	ids: string[];
	all: boolean;
};

export const emptySelection: Selection = { ids: [], all: false };

/** Whether one row is inside the selection. An all-selection contains every row it loaded. */
export function rowChecked(selection: Selection, id: string): boolean {
	return selection.all || selection.ids.includes(id);
}

/** Take a row out of, or back into, the selection. Deselecting under Select-All drops to ids. */
export function toggleRow(
	selection: Selection,
	id: string,
	checked: boolean,
	loaded: string[]
): Selection {
	if (selection.all) {
		if (checked) return selection;
		return { ids: loaded.filter((entry) => entry !== id), all: false };
	}
	return {
		...selection,
		ids: checked ? [...selection.ids, id] : selection.ids.filter((entry) => entry !== id)
	};
}

/** Select-All replaces an id set outright, and clearing empties both halves at once. */
export function toggleAll(checked: boolean): Selection {
	return checked ? { ids: [], all: true } : emptySelection;
}
