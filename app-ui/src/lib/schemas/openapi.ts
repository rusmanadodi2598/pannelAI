// The served v1 contract, as the panel reads it (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// §6.12 forbids a hand-written second copy of the contract, because a copy drifts and then lies. So this
// module declares only the part of `GET /api/v1/openapi.json` the screen reads, and the derivations live
// in `openapi-catalog.ts`.
//
// Required fields are strict and additions are tolerated, per §7.4: the document belongs to the gateway,
// so a new top-level block, a new path-level key, or a new operation key must not break this screen. A
// document that omits a block the screen would like (servers, tags, x-contract) is still a valid
// document, and the screen states the absence rather than inventing a value.

import { z } from 'zod';

const schemaSecurityRequirement = z.record(z.string(), z.array(z.string()));

export const schemaSecurityScheme = z.object({
	type: z.string(),
	in: z.string().optional(),
	name: z.string().optional(),
	scheme: z.string().optional(),
	bearerFormat: z.string().optional(),
	description: z.string().optional()
});

export type SecurityScheme = z.infer<typeof schemaSecurityScheme>;

const schemaOperation = z.object({
	summary: z.string().optional(),
	operationId: z.string().optional(),
	tags: z.array(z.string()).optional(),
	security: z.array(schemaSecurityRequirement).optional()
});

// The path-level keys OpenAPI defines are declared so a document that uses one still parses. The screen
// reads the method keys only, and a path-level `parameters` array is shared across methods rather than
// belonging to the one operation a table row shows.
const schemaPathItem = z.object({
	get: schemaOperation.optional(),
	post: schemaOperation.optional(),
	put: schemaOperation.optional(),
	patch: schemaOperation.optional(),
	delete: schemaOperation.optional(),
	head: schemaOperation.optional(),
	options: schemaOperation.optional(),
	trace: schemaOperation.optional(),
	parameters: z.array(z.unknown()).optional(),
	summary: z.string().optional(),
	description: z.string().optional()
});

const schemaResponse = z.object({
	description: z.string().optional(),
	'x-error-codes': z.array(z.string()).optional(),
	content: z
		.record(z.string(), z.object({ schema: z.object({ $ref: z.string().optional() }).optional() }))
		.optional()
});

const schemaPlane = z.object({
	security: z.array(z.string()).optional(),
	error_envelope: z.string().optional(),
	codes: z.record(z.string(), z.number()).optional()
});

export const schemaOpenAPIDocument = z.object({
	openapi: z.string(),
	info: z.object({
		title: z.string(),
		version: z.string(),
		summary: z.string().optional(),
		description: z.string().optional()
	}),
	servers: z.array(z.object({ url: z.string(), description: z.string().optional() })).optional(),
	security: z.array(schemaSecurityRequirement).optional(),
	tags: z.array(z.object({ name: z.string(), description: z.string().optional() })).optional(),
	paths: z.record(z.string(), schemaPathItem),
	components: z
		.object({
			securitySchemes: z.record(z.string(), schemaSecurityScheme).optional(),
			responses: z.record(z.string(), schemaResponse).optional()
		})
		.optional(),
	'x-contract': z.object({ planes: z.record(z.string(), schemaPlane).optional() }).optional()
});

export type OpenAPIDocument = z.infer<typeof schemaOpenAPIDocument>;
export type OpenAPIOperation = z.infer<typeof schemaOperation>;
export type OpenAPIResponse = z.infer<typeof schemaResponse>;
