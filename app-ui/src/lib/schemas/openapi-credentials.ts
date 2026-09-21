// Credentials, and the example call that shows one (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// §6.12 asks for the session-versus-gateway-key distinction and for one curl example per group. The
// example lives here rather than in the catalog module because its one variable part is the credential
// header: the method and path are the operation's own, and the only thing the panel decides is how to
// write a credential it must never print.
//
// Every placeholder below is deliberate. §6.12 requires the copied example to carry an `sk-...`
// placeholder and never a real key, so no branch reads a key from anywhere: it writes the shape.

import type { ApiOperation } from './openapi-catalog';
import { documentOperations } from './openapi-catalog';
import type { OpenAPIDocument, SecurityScheme } from './openapi';

/**
 * The panel's word for a credential. The two names are the document's own scheme names; a name the panel
 * does not know is printed verbatim, so a scheme added to the contract shows up as itself rather than as
 * a guess. Several schemes are joined with `or`, which is what a list of requirements means.
 */
export function credentialLabel(schemes: string[]): string {
	if (schemes.length === 0) return 'No credential';
	return schemes.map(schemeLabel).join(' or ');
}

const SCHEME_LABELS: Record<string, string> = {
	sessionCookie: 'Session cookie',
	gatewayKey: 'Gateway key'
};

function schemeLabel(name: string): string {
	return SCHEME_LABELS[name] ?? name;
}

/** Where a scheme's credential travels, derived from the scheme's own declaration. */
export function schemePlacement(scheme: SecurityScheme): string {
	if (scheme.type === 'apiKey') {
		const where = scheme.in ?? 'header';
		return scheme.name ? `${where} ${scheme.name}` : where;
	}
	if (scheme.type === 'http') {
		const kind = scheme.scheme ?? 'http';
		return scheme.bearerFormat ? `${kind} (${scheme.bearerFormat})` : kind;
	}
	return scheme.type;
}

/**
 * The header an example call carries for one scheme. A scheme the document does not define falls back to
 * the scheme's own name, which is a placeholder a reader can act on rather than a header the panel
 * invented.
 */
export function credentialHeader(name: string, scheme?: SecurityScheme): string {
	if (scheme?.type === 'apiKey' && scheme.in === 'cookie') {
		return `Cookie: ${scheme.name ?? name}=<session cookie>`;
	}
	if (scheme?.type === 'http' && scheme.scheme === 'bearer') return 'Authorization: Bearer sk-...';
	if (scheme?.type === 'http' && scheme.scheme === 'basic')
		return 'Authorization: Basic <credential>';
	if (scheme?.type === 'apiKey' && scheme.name) return `${scheme.name}: <credential>`;
	return `${name}: <credential>`;
}

/** A path with its parameter braces replaced, so no example prints a literal `{id}` as a URL. */
export function examplePath(path: string): string {
	return path.replace(/\{([^}]+)\}/g, '<$1>');
}

/** The base URL an example is built on. The placeholder is stated when the document declares none. */
export function documentBaseUrl(doc: OpenAPIDocument): string {
	return (doc.servers?.[0]?.url ?? '').replace(/\/+$/, '') || '<gateway base URL>';
}

/** One example call, composed from the operation's own method, path, and credential. */
export function curlExample(operation: ApiOperation, doc: OpenAPIDocument): string {
	const url = `${documentBaseUrl(doc)}${examplePath(operation.path)}`;
	const scheme = operation.schemes[0];
	const header = scheme ? credentialHeader(scheme, doc.components?.securitySchemes?.[scheme]) : '';

	const lines = [`curl -X ${operation.method.toUpperCase()} '${url}'`];
	if (header) lines.push(`  -H '${header}'`);
	return lines.join(' \\\n');
}

export type SchemeUse = {
	name: string;
	/** Absent when an operation names a scheme the document does not define. */
	scheme?: SecurityScheme;
	operations: number;
};

/** Every scheme the document defines or an operation names, with how many operations use each. */
export function schemeUses(doc: OpenAPIDocument): SchemeUse[] {
	const definitions = doc.components?.securitySchemes ?? {};
	const counts = new Map<string, number>();

	for (const operation of documentOperations(doc)) {
		for (const name of operation.schemes) counts.set(name, (counts.get(name) ?? 0) + 1);
	}

	const uses: SchemeUse[] = Object.entries(definitions).map(([name, scheme]) => ({
		name,
		scheme,
		operations: counts.get(name) ?? 0
	}));

	for (const [name, count] of counts) {
		if (!(name in definitions)) uses.push({ name, operations: count });
	}

	return uses;
}

/** How many operations declare no credential at all. */
export function publicOperationCount(doc: OpenAPIDocument): number {
	return documentOperations(doc).filter((operation) => operation.schemes.length === 0).length;
}
