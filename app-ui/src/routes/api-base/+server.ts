// The gateway base address route (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// Thin by design: the rule lives in `src/lib/server/api-base.ts`, which is what the test drives. The
// address is outside `/api/v1` because it is the panel's own answer, and anything under the forwarder's
// prefix would be sent to the gateway instead of being served here.

import type { RequestHandler } from './$types';
import { apiBaseResponse } from '$lib/server/api-base';

export const GET: RequestHandler = ({ request, fetch }) =>
	apiBaseResponse(request.headers.get('cookie'), fetch);
