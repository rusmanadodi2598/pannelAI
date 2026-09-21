// A small stand-in for the served contract, for the API Docs tests.
//
// Not a copy of `app-serv/internal/handler/openapi.json`: that artifact is generated, 300 KB, and owned
// by the other app. This fixture is the shape the panel reads, small enough to read in a diff, and the
// live pass is what proves the panel parses the real document.
//
// Every branch the transforms have to handle is present here: a public operation, an operation that
// inherits the document's credential, a path parameter, a group that mixes credentials, an untagged
// operation, and a code the document declares without describing.

export function openapiDocument(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		openapi: '3.1.0',
		info: {
			title: 'pannelAI API',
			version: 'v0.4.0',
			summary: 'Management and data-plane API for the pannelAI gateway.',
			description:
				'Management routes use the pannel_session cookie. Data-plane routes use a gateway key.'
		},
		servers: [{ url: 'http://localhost:8080', description: 'Local app-serv' }],
		security: [{ sessionCookie: [] }],
		tags: [{ name: 'System' }, { name: 'Gateway Keys' }, { name: 'Data Plane' }],
		paths: {
			'/api/v1/health': {
				get: { summary: 'Liveness and DB reachability', tags: ['System'], security: [] }
			},
			'/api/v1/gateway-keys': {
				get: { summary: 'List keys', tags: ['Gateway Keys'] },
				post: { summary: 'Create a key', tags: ['Gateway Keys'] }
			},
			'/api/v1/gateway-keys/{id}': {
				patch: { summary: 'Rename a key', tags: ['Gateway Keys'] }
			},
			'/api/v1/chat/completions': {
				post: { summary: 'Chat', tags: ['Data Plane'], security: [{ gatewayKey: [] }] }
			},
			'/api/v1/settings': {
				get: { summary: 'Read settings' }
			}
		},
		components: {
			securitySchemes: {
				sessionCookie: {
					type: 'apiKey',
					in: 'cookie',
					name: 'pannel_session',
					description: 'Dashboard session cookie.'
				},
				gatewayKey: {
					type: 'http',
					scheme: 'bearer',
					bearerFormat: 'gateway-key',
					description: 'Gateway key created through the management API.'
				}
			},
			responses: {
				ManagementValidationError: {
					description: 'The request body or query failed validation.',
					'x-error-codes': ['VALIDATION_ERROR'],
					content: {
						'application/json': { schema: { $ref: '#/components/schemas/ManagementError' } }
					}
				},
				ManagementUnauthorizedError: {
					description: 'No valid session cookie was presented.',
					'x-error-codes': ['UNAUTHORIZED'],
					content: {
						'application/json': { schema: { $ref: '#/components/schemas/ManagementError' } }
					}
				},
				DataPlaneUnauthorizedError: {
					description: 'The gateway key was missing or invalid.',
					'x-error-codes': ['UNAUTHORIZED'],
					content: {
						'application/json': { schema: { $ref: '#/components/schemas/DataPlaneError' } }
					}
				}
			}
		},
		'x-contract': {
			planes: {
				management: {
					security: ['sessionCookie'],
					error_envelope: 'ManagementError',
					codes: { UNAUTHORIZED: 401, VALIDATION_ERROR: 400 }
				},
				data_plane: {
					security: ['gatewayKey'],
					error_envelope: 'DataPlaneError',
					codes: { MODEL_NOT_FOUND: 400, UNAUTHORIZED: 401 }
				}
			}
		},
		...overrides
	};
}
