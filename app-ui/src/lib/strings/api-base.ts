// API base copy (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// One module for the dialog's own prose, so a wording change is one edit and no sentence is composed
// inside a component. English only, no em dash (R-02), no marketing vocabulary (R-16), and no figure the
// panel did not measure (R-17).
//
// The address is the one the panel server answers with, so both snippets are composed from it rather than
// written out, and a panel restarted against a different gateway gets both lines right without an edit
// here. The key in them is a placeholder, not a credential: the panel holds no gateway key, and the
// sentence under each snippet says where the real one comes from.

export const API_BASE_COPY = {
	title: 'API base',
	tabsLabel: 'API base formats',
	intro: 'Where a client sends its calls, and the two forms a client is usually configured with.',

	tabs: {
		baseUrl: 'Base URL',
		curl: 'cURL',
		openai: 'OpenAI client'
	},

	loading: 'Reading the gateway address from the panel server.',
	retry: 'Try again',

	baseUrl: {
		note: 'The gateway address this panel server forwards /api/v1 to, read from the panel configuration rather than from the browser. A client on another machine substitutes the host name this gateway answers on for a loopback address here.'
	},

	curl: {
		/** One line, so the copied command runs as it stands. */
		command: (base: string) => `curl -H "Authorization: Bearer sk-..." ${base}/models`,
		note: 'Replace sk-... with a gateway key from Endpoint & Key. This route answers with the models that key can route, so it is also the shortest way to check one.'
	},

	openai: {
		/** The two names an OpenAI-compatible client reads, one per line. */
		env: (base: string) => `OPENAI_BASE_URL=${base}\nOPENAI_API_KEY=sk-...`,
		note: 'The gateway serves its OpenAI, Anthropic, and Responses wires on this base, and a client appends its own path to it.'
	}
} as const;
