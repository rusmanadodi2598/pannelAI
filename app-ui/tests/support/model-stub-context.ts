// What a model-stub route handler reads (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The stub is one fake fetch split by the resource a branch answers. Every handler takes this context and
// returns a response when the branch belongs to it, or null so the next family can look; the order the
// dispatcher calls them in is the order the single `if` chain had, because two branches can match one path.

import type { ModelStub, StubModel } from './model-stub-types';

export type StubContext = {
	stub: ModelStub;
	method: string;
	body: StubModel;
	parsed: URL;
	json: (payload: unknown, status?: number) => Response;
	refusal: (code: string, message: string, status: number) => Response;
	key: (ref: StubModel) => string;
	blocked: () => Set<string>;
	catalogWithCustom: () => StubModel[];
};
