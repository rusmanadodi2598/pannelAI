// The unsaved-draft guard (docs/SPEC-UI/001-SPEC-UI.md §8.4.4).
//
// A form that holds an unsaved draft registers its dirty state here, and the root layout is the one
// place that asks before a navigation takes that draft away. Two cases are kept apart on purpose: a
// navigation that unloads the document gets the browser's own confirmation, which is raised by
// cancelling the navigation rather than by asking again here, while a navigation inside the panel has
// no browser dialog, so the guard asks with `confirm`. That call is synchronous, which is what
// `beforeNavigate` can use; a custom modal would have to defer the navigation, and it would be a
// second dialog language for a rule the browser already answers with its own.
//
// The registry reads each form's dirty state when a navigation happens, not when the form registers,
// so a draft that was saved or discarded stops warning without any bookkeeping in the form.

/** What the operator is asked before a draft is left behind. One sentence, and it names the loss. */
export const LEAVE_WARNING = 'You have unsaved changes on this screen. Leave and discard them?';

const forms = new Set<() => boolean>();

/** Registers one form's dirty state. The returned function removes the form again. */
export function registerDirtyForm(isDirty: () => boolean): () => void {
	forms.add(isDirty);
	return () => {
		forms.delete(isDirty);
	};
}

/** Whether any registered form holds an unsaved draft right now. */
export function hasDirtyForm(): boolean {
	for (const isDirty of forms) {
		if (isDirty()) return true;
	}
	return false;
}

/**
 * Whether a navigation must be cancelled to protect an unsaved draft. `confirmLeave` is injected so a
 * test can answer the question without a browser dialog.
 */
export function shouldCancelNavigation(
	navigation: { willUnload: boolean },
	confirmLeave: (message: string) => boolean
): boolean {
	if (!hasDirtyForm()) return false;

	// The browser asks this one itself, so asking here would prompt twice for one leave.
	if (navigation.willUnload) return true;

	return !confirmLeave(LEAVE_WARNING);
}
