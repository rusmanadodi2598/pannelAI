// Skills copy (docs/SPEC-UI/001-SPEC-UI.md §6.10, §8.10.1).
//
// One module for this screen's own prose, so a wording change is one edit and no sentence is composed
// inside a component. English only, no em dash (R-02), no marketing vocabulary (R-16), and no figure the
// catalog did not provide (R-17): every count here is passed in.
//
// The four causes are separate sentences because the operator's next move differs: a file that is not
// published is a push, a host that answered something else is a different problem, and a request that
// never completed is worth pressing again.

import type { SourceCause } from '$lib/schemas/skill-source';

export const SKILLS_COPY = {
	title: 'Skills',
	subtitle:
		'One document per capability, for an AI client to read. The gateway serves the catalog; the documents live in the product repository.',

	catalog: {
		/** The catalog names itself and its route; the panel only labels them. */
		readFrom: (path: string) => `Read from GET ${path}.`,
		rows: (count: number) => (count === 1 ? '1 row' : `${count} rows`),
		loading: 'Loading the skill catalog',
		errorTitle: 'The skill catalog could not be read',
		retry: 'Try again',
		emptyTitle: 'The gateway serves no skill',
		emptyDescription:
			'The catalog answered with no rows, so there is nothing to hand out here. That is a fact about the catalog, not a filter on this screen.'
	},

	entry: {
		heading: 'Paste this into an AI client',
		intro: 'The entry skill indexes every other one, so this single line is enough to start.',
		/** Shown when the catalog carries no row marked `entry`. */
		absent:
			'This catalog carries no entry skill, so there is no single line to hand out. The rows below are the capabilities it does carry.'
	},

	sources: {
		heading: 'Capability skills',
		intro:
			'One row per capability. The agent address is what a client fetches; the GitHub link is for reading.',
		/**
		 * How many of the catalog's sources are published, measured rather than claimed. The number is
		 * the published count rather than the answered count: every probe answers, so "answered" would
		 * read as "7 of 7" while seven rows report a 404, and a count that cannot move tells the
		 * operator nothing about the thing they are looking at.
		 */
		summary: (published: number, total: number) =>
			published === 1
				? `1 of ${total} sources is published at the ref the catalog names.`
				: `${published} of ${total} sources are published at the ref the catalog names.`,
		checking: (total: number) => (total === 1 ? 'Checking 1 source' : `Checking ${total} sources`),
		checkAgain: 'Check again',
		checkingRow: 'Checking the source',
		available: 'Source available',
		unavailable: 'Source unavailable',
		agentAddress: 'Agent address',
		readOnGitHub: 'Read it on GitHub',
		copyInstall: 'Copy install line',
		/** A row that teaches no single route, stated as the fact rather than left blank. */
		noEndpoint: 'No endpoint'
	},

	cause: {
		missing: (status: number) =>
			`The source host has no file at that path (HTTP ${status}), so this document is not published at the ref the catalog names. Publishing it is a change in the product repository, not in this panel.`,
		http: (status: number) => `The source host answered HTTP ${status} for that path.`,
		timeout: (seconds: number) =>
			`The source host did not answer within ${seconds} seconds, so the row is left unreported rather than called broken.`,
		network: (detail: string) =>
			`The request to the source host did not complete: ${detail}. This panel asks from the browser, so an offline network or a blocking proxy reads the same way.`
	}
} as const;

/** One sentence for a probe that did not answer. */
export function causeSentence(cause: SourceCause): string {
	switch (cause.kind) {
		case 'missing':
			return SKILLS_COPY.cause.missing(cause.status);
		case 'http':
			return SKILLS_COPY.cause.http(cause.status);
		case 'timeout':
			return SKILLS_COPY.cause.timeout(cause.seconds);
		case 'network':
			return SKILLS_COPY.cause.network(cause.detail);
	}
}
