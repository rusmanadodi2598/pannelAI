// The panel root has no content of its own.
//
// It redirects to the first item in the owner KEEP list rather than to a summary page that the spec
// does not define (docs/SPEC-UI/001-SPEC-UI.md §5.1).

import { redirect } from '@sveltejs/kit';

export function load(): never {
	redirect(307, '/endpoint-keys');
}
