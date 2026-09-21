// The catalog, derived from the served v1 contract (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// The screen shows one section per group and one row per operation. Both come from the document: the
// group order is the document's tag order, and a row is an operation the document lists. Nothing here
// restates the contract, so a document that grows a tag or a path renders with no edit to this file,
// which is what keeps §6.12's "no second copy" rule from decaying.

import type { OpenAPIDocument } from './openapi';

export const HTTP_METHODS = [
	'get',
	'post',
	'put',
	'patch',
	'delete',
	'head',
	'options',
	'trace'
] as const;

export type HttpMethod = (typeof HTTP_METHODS)[number];

/** The group an operation lands in when the document gives it no tag. */
export const UNTAGGED_GROUP = 'Untagged';

export type ApiOperation = {
	method: HttpMethod;
	path: string;
	summary?: string;
	tags: string[];
	/** The scheme names that satisfy this operation. Empty means the document requires no credential. */
	schemes: string[];
};

export type ApiGroup = { name: string; operations: ApiOperation[] };

/** Every operation, in the document's own path order and in the method order above. */
export function documentOperations(doc: OpenAPIDocument): ApiOperation[] {
	const operations: ApiOperation[] = [];

	for (const [path, item] of Object.entries(doc.paths)) {
		for (const method of HTTP_METHODS) {
			const operation = item[method];
			if (!operation) continue;

			operations.push({
				method,
				path,
				summary: operation.summary,
				tags: operation.tags ?? [],
				// An operation that declares no `security` inherits the document's, which is what the
				// document means by the block; `[]` is the document saying "no credential" and survives
				// the fallback because an empty array is not nullish.
				schemes: requirementNames(operation.security ?? doc.security)
			});
		}
	}

	return operations;
}

function requirementNames(requirements?: Record<string, string[]>[]): string[] {
	const names = new Set<string>();
	for (const requirement of requirements ?? []) {
		for (const name of Object.keys(requirement)) names.add(name);
	}
	return [...names];
}

/**
 * Groups in the document's own tag order, then any tag only an operation declares, then the untagged
 * tail. A declared tag with no operation is dropped rather than rendered as an empty section.
 */
export function documentGroups(doc: OpenAPIDocument): ApiGroup[] {
	const groups = new Map<string, ApiOperation[]>();
	for (const tag of doc.tags ?? []) groups.set(tag.name, []);

	for (const operation of documentOperations(doc)) {
		// An operation belongs to every tag it declares, which is what a tag means; one that declares
		// none is listed under a name that says so instead of being dropped from the catalog.
		for (const tag of operation.tags.length > 0 ? operation.tags : [UNTAGGED_GROUP]) {
			const group = groups.get(tag);
			if (group) group.push(operation);
			else groups.set(tag, [operation]);
		}
	}

	return [...groups.entries()]
		.filter(([, operations]) => operations.length > 0)
		.map(([name, operations]) => ({ name, operations }));
}

/** A stable in-page anchor for a group heading. */
export function groupAnchor(name: string): string {
	const slug = name
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
	return slug.length > 0 ? `group-${slug}` : 'group';
}

/**
 * The call a group's example shows: its first read when it has one, because a GET is a call the
 * operator can run as printed, and otherwise its first operation.
 */
export function groupExample(group: ApiGroup): ApiOperation | undefined {
	return group.operations.find((operation) => operation.method === 'get') ?? group.operations[0];
}
