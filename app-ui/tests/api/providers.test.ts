import { afterEach, describe, expect, it, vi } from 'vitest';
import { listProviders } from '$lib/api/providers';

describe('provider registry API', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('rejects a search query beyond the API limit before sending a request', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);

		const result = await listProviders({ q: 'x'.repeat(121) });

		expect(result.ok).toBe(false);
		if (!result.ok) expect(result.error.code).toBe('VALIDATION_ERROR');
		expect(fetchMock).not.toHaveBeenCalled();
	});
});
