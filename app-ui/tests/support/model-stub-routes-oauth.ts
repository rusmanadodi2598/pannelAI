// The OAuth branches of the model stub (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// The device round is its own route family: start hands back the vendor's page and code, and poll
// answers in the order the test declared until the vendor grants the token or refuses.

import type { StubContext } from './model-stub-context';

export function handleOAuthRoutes({
	stub,
	method,
	body,
	parsed,
	json,
	refusal
}: StubContext): Response | null {
	const oauthMatch = /\/providers\/([^/]+)\/oauth\/(status|start|refresh)$/.exec(parsed.pathname);

	if (method === 'GET' && oauthMatch?.[2] === 'status') {
		stub.reads.push('oauth:status');
		if (stub.oauthReadStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The oauth state could not be read.', stub.oauthReadStatus);
		}
		return json({
			// The provider the route names, so a test cannot pass against an answer about another one.
			provider_id: decodeURIComponent(oauthMatch[1]),
			flow: stub.oauthFlow,
			endpoints: stub.oauthEndpoints
		});
	}

	if (method === 'POST' && oauthMatch?.[2] === 'start') {
		stub.oauthStarts.push(decodeURIComponent(oauthMatch[1]));
		if (stub.oauthStartRefusal) {
			const { status, code, message } = stub.oauthStartRefusal;
			return refusal(code, message, status);
		}
		return json({ authorize_url: 'https://provider.test/authorize?state=st_1', state: 'st_1' });
	}

	if (method === 'POST' && oauthMatch?.[2] === 'refresh') {
		const endpointId = (body.endpoint_id as string | undefined) ?? null;
		stub.oauthRefreshes.push(endpointId);
		if (stub.oauthRefreshRefusal) {
			const { status, code, message } = stub.oauthRefreshRefusal;
			return refusal(code, message, status);
		}

		// A real refresh moves the token: the named account, or every due one, becomes fresh. The panel
		// re-reads the status afterwards, so a stub that answered without moving anything would let a
		// panel that never re-reads pass.
		const moved: string[] = [];
		stub.oauthEndpoints = stub.oauthEndpoints.map((endpoint) => {
			const due = endpoint.refresh_state === 'due';
			if (endpointId !== null ? endpoint.endpoint_id !== endpointId : !due) return endpoint;
			moved.push(String(endpoint.endpoint_id));
			return {
				...endpoint,
				refresh_state: 'fresh',
				last_refresh_at: '2026-09-20T12:00:00Z'
			};
		});

		return json({ refreshed: moved.length, endpoint_ids: moved });
	}

	const deviceMatch = /\/providers\/([^/]+)\/oauth\/device\/(start|poll)$/.exec(parsed.pathname);

	if (method === 'POST' && deviceMatch?.[2] === 'start') {
		stub.oauthDeviceStarts.push(decodeURIComponent(deviceMatch[1]));
		if (stub.oauthDeviceStartRefusal) {
			const { status, code, message } = stub.oauthDeviceStartRefusal;
			return refusal(code, message, status);
		}
		return json(stub.oauthDeviceStart);
	}

	if (method === 'POST' && deviceMatch?.[2] === 'poll') {
		stub.oauthDevicePolls.push({ device_code: (body.device_code as string | undefined) ?? '' });
		const ask = stub.oauthDevicePolls.length;
		if (stub.oauthDevicePollRefusalAt > 0 && ask === stub.oauthDevicePollRefusalAt) {
			const { status, code, message } = stub.oauthDevicePollRefusal ?? {
				status: 502,
				code: 'UPSTREAM_ERROR',
				message: 'The device token endpoint refused the poll.'
			};
			return refusal(code, message, status);
		}
		// Each ask consumes one scripted answer; past the end of the queue the round is still
		// unanswered, which is what `pending` means on the wire too.
		return json(stub.oauthDevicePollAnswers.shift() ?? { status: 'pending' });
	}

	return null;
}
