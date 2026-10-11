import { afterEach, describe, expect, it, vi } from 'vitest';
import { listProviders } from '$lib/api/providers';

describe('provider registry API', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('enforces the API search limit by Unicode code points', async () => {
		const cases = [
			{ name: '120 ASCII code points', query: 'x'.repeat(120), accepted: true },
			{ name: '121 ASCII code points', query: 'x'.repeat(121), accepted: false },
			{ name: '120 astral Unicode code points', query: '😀'.repeat(120), accepted: true },
			{ name: '121 astral Unicode code points', query: '😀'.repeat(121), accepted: false }
		];
		for (const testCase of cases) {
			vi.unstubAllGlobals();
			const { query, accepted } = testCase;
			const fetchMock = vi.fn(
				async () =>
					new Response(JSON.stringify({ data: [], meta: { page: 1, per_page: 25, total: 0 } }), {
						status: 200,
						headers: { 'content-type': 'application/json' }
					})
			);
			vi.stubGlobal('fetch', fetchMock);

			const result = await listProviders({ q: query });

			expect(result.ok).toBe(accepted);
			if (!accepted && !result.ok) expect(result.error.code).toBe('VALIDATION_ERROR');
			expect(fetchMock).toHaveBeenCalledTimes(accepted ? 1 : 0);
		}
	});
});
