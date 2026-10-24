// The parsed row shapes the bulk-selection tests render (docs/SPEC-UI/001-SPEC-UI.md §6.2, §6.9).
//
// The row builders answer the wire shape and these components take the parsed one. Casting at the fixture
// boundary keeps the builders shared rather than duplicated for the typed call sites.

import { endpointKeyRow, endpointRow } from './endpoint-stub';
import { proxyRow } from './proxy-stub';
import type { Endpoint, EndpointKey } from '$lib/schemas/endpoint';
import type { Proxy } from '$lib/schemas/proxy';

export const endpointKey = (overrides: Record<string, unknown> = {}): EndpointKey =>
	endpointKeyRow(overrides) as unknown as EndpointKey;

export const endpoint = (overrides: Record<string, unknown> = {}): Endpoint =>
	endpointRow(overrides) as unknown as Endpoint;

export const proxy = (overrides: Record<string, unknown> = {}): Proxy =>
	proxyRow(overrides) as unknown as Proxy;
