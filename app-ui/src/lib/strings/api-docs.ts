// API Docs copy (docs/SPEC-UI/001-SPEC-UI.md §6.12, §8.10.1).
//
// One module for this screen's own prose, so a wording change is one edit and no sentence is composed
// inside a component. English only, no em dash (R-02), no marketing vocabulary (R-16), and no figure the
// document did not provide (R-17): every count this file phrases is passed in from the document.

export const API_DOCS_COPY = {
	title: 'API Docs',
	subtitle: 'The v1 contract this gateway serves, read from the gateway itself.',

	document: {
		/** The document names itself and its version; the panel only labels them. */
		version: (version: string) => `Document version ${version}`,
		readFrom: (path: string) => `Read from GET ${path}.`,
		loading: 'Loading the served contract',
		errorTitle: 'The contract could not be read',
		retry: 'Try again',
		emptyTitle: 'This document declares no paths',
		emptyDescription:
			'The gateway answered with a contract that lists no operation, so there is nothing to render here. That is a fact about the document, not a filter on this screen.'
	},

	baseUrl: {
		heading: 'Base URL',
		/** Shown when the document carries no servers block. */
		absent:
			'This document declares no base URL, so the examples below show a placeholder. Ask the gateway operator for the address a client should call.'
	},

	credentials: {
		heading: 'Credentials',
		/** Shown when the document carries no securitySchemes. */
		absent: 'This document defines no credential scheme, so no call carries one.',
		usage: (count: number) =>
			count === 1 ? 'Used by 1 operation.' : `Used by ${count} operations.`,
		undefined: 'Named by an operation but not defined in this document.',
		publicOperations: (count: number) =>
			count === 1
				? 'One operation declares no credential.'
				: `${count} operations declare no credential.`
	},

	catalog: {
		heading: 'Endpoints',
		intro: 'Every operation the gateway registers, grouped the way the document groups it.',
		indexLabel: 'Groups',
		operations: (count: number) => (count === 1 ? '1 operation' : `${count} operations`),
		columns: {
			method: 'Method',
			path: 'Path',
			summary: 'Summary',
			credential: 'Credential'
		},
		example: 'Example call',
		noSummary: 'The document gives this operation no summary.'
	},

	errors: {
		heading: 'Error codes',
		/** Shown when the document carries no x-contract block. */
		absent:
			'This document carries no error code table, so the codes are not listed here. The contract publishes them in its x-contract block.',
		envelope: (name: string) => `Failures arrive as ${name}.`
	}
} as const;
