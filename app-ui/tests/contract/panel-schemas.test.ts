// The panel's response schemas against the document the gateway serves (AGENTS.md §2.4, SPEC-API §4).
//
// README item 1: the contract was pinned on the service side, and the panel side of a field rename was caught
// only by the resource's own fixture. This reads the committed `openapi.json` (the artifact the YAML gate
// pins to the contract of record) and asks one question of every response schema the panel parses: does the
// contract name each field the panel requires?
//
// It compares property names rather than full shapes, which is the honest limit of this gate: a field renamed
// or removed on the wire fails here, and a field whose type changed does not. Types are the resource tests'
// job, because each of them holds a fixture of the real answer.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import type { z } from 'zod';
import {
	schemaProvider,
	schemaProviderDetail,
	schemaProviderList,
	schemaProviderMedia,
	schemaProviderModel,
	schemaProviderModelList,
	schemaProviderStatusSummary
} from '$lib/schemas/provider';
import {
	schemaEndpoint,
	schemaEndpointAccount,
	schemaEndpointKey,
	schemaEndpointKeyList,
	schemaEndpointLabel,
	schemaEndpointLabelList,
	schemaEndpointList,
	schemaEndpointOAuth,
	schemaEndpointTestStatus
} from '$lib/schemas/endpoint';
import { schemaCombo, schemaComboList, schemaComboModelEntry } from '$lib/schemas/combo';
import { schemaCatalogModel, schemaModelCatalog } from '$lib/schemas/model';
import { schemaQuotaWindow, schemaQuotaWindowList } from '$lib/schemas/quota';
import {
	schemaUsageBucket,
	schemaUsageGroup,
	schemaUsageRecord,
	schemaUsageRecordDetail,
	schemaUsageRecordList,
	schemaUsageSummary,
	schemaUsageTimeseries,
	schemaUsageTotals
} from '$lib/schemas/usage';
import {
	schemaConsoleLog,
	schemaLogDetail,
	schemaLogList,
	schemaLogPurge,
	schemaLogRecord
} from '$lib/schemas/log';
import {
	schemaCreatedGatewayKey,
	schemaGatewayKey,
	schemaGatewayKeyList
} from '$lib/schemas/gateway-key';
import {
	schemaProxy,
	schemaProxyList,
	schemaProxyTest,
	schemaProxyTestStatus
} from '$lib/schemas/proxy';
import { schemaChangelog, schemaChangelogEntry } from '$lib/schemas/changelog';
import { schemaSkill } from '$lib/schemas/skill';

const DOCUMENT = resolve(process.cwd(), '../app-serv/internal/handler/openapi.json');

/** Every property name the served document declares, at any depth. */
function documentedPropertyNames(): Set<string> {
	const document = JSON.parse(readFileSync(DOCUMENT, 'utf-8')) as {
		components?: { schemas?: Record<string, unknown> };
	};
	const names = new Set<string>();

	const walk = (node: unknown): void => {
		if (Array.isArray(node)) {
			for (const item of node) walk(item);
			return;
		}
		if (node === null || typeof node !== 'object') return;

		for (const [key, value] of Object.entries(node as Record<string, unknown>)) {
			if (key === 'properties' && value !== null && typeof value === 'object') {
				for (const name of Object.keys(value)) names.add(name);
			}
			walk(value);
		}
	};

	walk(document.components?.schemas ?? {});

	return names;
}

/** The top-level keys a panel schema refuses to do without. */
function requiredKeys(schema: z.ZodType): string[] {
	const shape = (schema as unknown as { shape?: Record<string, z.ZodType> }).shape;
	if (shape === undefined) return [];

	return Object.entries(shape)
		.filter(([, field]) => !field.safeParse(undefined).success)
		.map(([key]) => key);
}

const RESPONSE_SCHEMAS: { name: string; schema: z.ZodType }[] = [
	{ name: 'schemaProvider', schema: schemaProvider },
	{ name: 'schemaProviderDetail', schema: schemaProviderDetail },
	{ name: 'schemaProviderStatusSummary', schema: schemaProviderStatusSummary },
	{ name: 'schemaProviderMedia', schema: schemaProviderMedia },
	{ name: 'schemaProviderModel', schema: schemaProviderModel },
	{ name: 'schemaProviderList', schema: schemaProviderList },
	{ name: 'schemaProviderModelList', schema: schemaProviderModelList },
	{ name: 'schemaEndpoint', schema: schemaEndpoint },
	{ name: 'schemaEndpointAccount', schema: schemaEndpointAccount },
	{ name: 'schemaEndpointOAuth', schema: schemaEndpointOAuth },
	{ name: 'schemaEndpointTestStatus', schema: schemaEndpointTestStatus },
	{ name: 'schemaEndpointKey', schema: schemaEndpointKey },
	{ name: 'schemaEndpointList', schema: schemaEndpointList },
	{ name: 'schemaEndpointKeyList', schema: schemaEndpointKeyList },
	{ name: 'schemaEndpointLabel', schema: schemaEndpointLabel },
	{ name: 'schemaEndpointLabelList', schema: schemaEndpointLabelList },
	{ name: 'schemaCombo', schema: schemaCombo },
	{ name: 'schemaComboModelEntry', schema: schemaComboModelEntry },
	{ name: 'schemaComboList', schema: schemaComboList },
	{ name: 'schemaCatalogModel', schema: schemaCatalogModel },
	{ name: 'schemaModelCatalog', schema: schemaModelCatalog },
	{ name: 'schemaQuotaWindow', schema: schemaQuotaWindow },
	{ name: 'schemaQuotaWindowList', schema: schemaQuotaWindowList },
	{ name: 'schemaUsageTotals', schema: schemaUsageTotals },
	{ name: 'schemaUsageGroup', schema: schemaUsageGroup },
	{ name: 'schemaUsageSummary', schema: schemaUsageSummary },
	{ name: 'schemaUsageBucket', schema: schemaUsageBucket },
	{ name: 'schemaUsageTimeseries', schema: schemaUsageTimeseries },
	{ name: 'schemaUsageRecord', schema: schemaUsageRecord },
	{ name: 'schemaUsageRecordList', schema: schemaUsageRecordList },
	{ name: 'schemaUsageRecordDetail', schema: schemaUsageRecordDetail },
	{ name: 'schemaLogRecord', schema: schemaLogRecord },
	{ name: 'schemaLogList', schema: schemaLogList },
	{ name: 'schemaLogDetail', schema: schemaLogDetail },
	{ name: 'schemaLogPurge', schema: schemaLogPurge },
	{ name: 'schemaConsoleLog', schema: schemaConsoleLog },
	{ name: 'schemaGatewayKey', schema: schemaGatewayKey },
	{ name: 'schemaGatewayKeyList', schema: schemaGatewayKeyList },
	{ name: 'schemaCreatedGatewayKey', schema: schemaCreatedGatewayKey },
	{ name: 'schemaProxy', schema: schemaProxy },
	{ name: 'schemaProxyList', schema: schemaProxyList },
	{ name: 'schemaProxyTest', schema: schemaProxyTest },
	{ name: 'schemaProxyTestStatus', schema: schemaProxyTestStatus },
	{ name: 'schemaChangelog', schema: schemaChangelog },
	{ name: 'schemaChangelogEntry', schema: schemaChangelogEntry },
	{ name: 'schemaSkill', schema: schemaSkill }
];

/**
 * The one divergence this gate knows about, recorded rather than waived.
 *
 * `EndpointOAuthResponse` in the served document declares `flow`, `scopes`, `expires_at` and
 * `account_email`; the service's own DTO (`app-serv/internal/schema/endpoint_response.go`) also serves
 * `has_access_token` and `has_refresh_token`. The panel is right and the contract is incomplete, which is
 * the service side's to fix. The third assertion below fails the day the contract names them, so this
 * entry cannot outlive the gap it records.
 */
const KNOWN_UNDOCUMENTED: Record<string, string[]> = {
	schemaEndpointOAuth: ['has_access_token', 'has_refresh_token']
};

describe('panel response schemas against the served contract', () => {
	const documented = documentedPropertyNames();

	it('reads a document with schemas, so an empty walk cannot pass silently', () => {
		expect(documented.size).toBeGreaterThan(100);
	});

	it('requires no field the contract does not document, beyond the recorded gap', () => {
		const missing = RESPONSE_SCHEMAS.flatMap(({ name, schema }) => {
			const known = new Set(KNOWN_UNDOCUMENTED[name] ?? []);
			return requiredKeys(schema)
				.filter((key) => !documented.has(key) && !known.has(key))
				.map((key) => `${name} requires "${key}"`);
		});

		expect(
			missing,
			`fields the panel reads that the served contract does not name:\n${missing.join('\n')}`
		).toEqual([]);
	});

	it('keeps no exception the contract has since closed', () => {
		const closed = Object.entries(KNOWN_UNDOCUMENTED).flatMap(([name, keys]) =>
			keys.filter((key) => documented.has(key)).map((key) => `${name}.${key} is documented now`)
		);

		expect(closed, `drop these from KNOWN_UNDOCUMENTED:\n${closed.join('\n')}`).toEqual([]);
	});
});
