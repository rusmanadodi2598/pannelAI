// The state shape the model stub carries (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Moved verbatim from model-stub.ts: every field is a knob a test can set, and the stub applies each
// one rather than echoing it, which is what makes the state rules testable.

export type StubModel = Record<string, unknown>;

export type ModelStub = {
	/** The merged catalog, as the registry plus custom rows would produce it. */
	catalog: StubModel[];
	/** The disabled set, every provider's pairs. */
	disabled: StubModel[];
	/** The custom rows, every provider's. */
	custom: StubModel[];
	/** The combos, whose names are a legal reference. */
	combos: StubModel[];
	/** The probe results `POST /combos/{id}/test` answers, one entry per stored reference. */
	comboTestResults: StubModel[];
	/** The test route's own status, for a test that needs the combo to look deleted. */
	comboTestStatus: number;
	/** The combo ids the test route was called with. */
	comboTests: string[];
	/** Every `provider/model` the panel asked to probe, in the order it asked. */
	modelTests: string[];
	/** Every sweep the panel asked for, with the limit it sent. */
	modelSweeps: { provider: string; limit: number }[];
	/** The rows the probe answers with. */
	modelTestRows: StubModel[];
	/** How many chat models the swept provider offers, which sets `tested` below `total`. */
	modelTestTotal: number | null;
	/** The sweep's own truncation notice, `deadline` when the budget ran out. */
	modelTestStopped: string | null;
	/** The status every probe route answers with, for a refusal case. */
	modelTestStatus: number;
	/** Whether the provider detail says this provider has OAuth, which is what renders the section. */
	hasOAuth: boolean;
	/**
	 * The flow `GET /oauth/status` reports, which is what decides what the section offers. The default is
	 * `code`, the flow that finishes by callback; a test that needs the device round sets `device`, and
	 * one that needs the dormant case sets `connector` (SPEC-UI §14 Q23).
	 */
	oauthFlow: string;
	/** The connected accounts the status route answers. */
	oauthEndpoints: StubModel[];
	/** The status route's own status, for a test that needs one read to fail. */
	oauthReadStatus: number;
	/** When set, the start route answers this refusal instead of an authorize URL. */
	oauthStartRefusal: { status: number; code: string; message: string } | null;
	/** When set, the refresh route answers this refusal instead of moving a token. */
	oauthRefreshRefusal: { status: number; code: string; message: string } | null;
	/** The provider ids the start route was called with. */
	oauthStarts: string[];
	/** The round `POST /oauth/device/start` answers; a test overwrites the fields it cares about. */
	oauthDeviceStart: StubModel;
	/** When set, the device start route answers this refusal instead of a round. */
	oauthDeviceStartRefusal: { status: number; code: string; message: string } | null;
	/**
	 * The answers `POST /oauth/device/poll` serves, one per ask, in order. An exhausted queue keeps
	 * answering `pending`, because that is what an unanswered round really is.
	 */
	oauthDevicePollAnswers: StubModel[];
	/** When above zero, that numbered ask (1-based) answers a refusal instead of a verdict. */
	oauthDevicePollRefusalAt: number;
	/** When set, the refusal `oauthDevicePollRefusalAt` answers. */
	oauthDevicePollRefusal: { status: number; code: string; message: string } | null;
	/** The endpoint ids the refresh route was called with, `null` for "every due account". */
	oauthRefreshes: (string | null)[];
	/** The provider ids the device start route was called with. */
	oauthDeviceStarts: string[];
	/** The bodies the device poll route was asked with, in order. */
	oauthDevicePolls: StubModel[];
	/** The provider ids the custom-model route knows, which is what makes an unknown one a refusal. */
	providers: string[];
	/**
	 * The provider list's rows, which the picker reads for its active set and its group names. Empty by
	 * default: a test that opens a picker sets the rows it wants offered.
	 */
	providerRows: StubModel[];
	disabledWrites: StubModel[][];
	customCreates: StubModel[];
	customDeletes: string[];
	reads: string[];
	readStatus: number;
	/** The disabled set's own status, for a test that needs one read to fail and another to succeed. */
	disabledReadStatus: number;
	/** What `GET /combos` reports as the total, when a test needs the page to look truncated. */
	combosTotal: number | null;
	writeStatus: number;
	/** The provider's own auth type, which is what decides whether the page offers the key dialog. */
	authType: string;
	/**
	 * The auth modes the provider declares (SPEC-API: `oauth`, `apikey`/`api_key`). Empty means "derive from
	 * `authType`/`hasOAuth`", which is what keeps an `authType: 'oauth'` test free of a stray key mode. A
	 * test that means a provider offering BOTH paths (Qoder: `['oauth','apikey']`) sets it explicitly.
	 */
	authModes: string[];
	/**
	 * The registry's credential-format sentence the detail row carries, or null for the omitempty
	 * shape where the key is absent entirely.
	 */
	authHint: string | null;
	/** The endpoint rows the Endpoints section lists. */
	endpoints: StubModel[];
	/**
	 * The connections list's own status, for a test that needs the label read to fail while the rest of the
	 * page loads. It is the same route, so the section behind the dialog reports the failure too.
	 */
	endpointReadStatus: number;
	/** The bodies `POST /endpoints` received, which is where a key name the route would refuse shows up. */
	endpointCreates: StubModel[];
	/** The endpoint ids `DELETE /endpoints/{id}` received, in order: what the connection list's removal calls. */
	endpointDeletes: string[];
	/** The delete's own status, for a test that needs the gateway to refuse a removal. */
	endpointDeleteStatus: number;
	/** The bodies `POST /endpoints/bulk` received, one per paste the screen sent as a batch. */
	endpointBulkCreates: StubModel[];
	/** The models `GET /providers/{id}/models` answers, which is what a custom node's import declares. */
	providerModels: StubModel[];
	/** The route's own status, for a test that needs the import's read to fail. */
	providerModelsReadStatus: number;
	/** The provider ids the route was called with, in order. */
	providerModelsReads: string[];
	/** When set, the answer carries this warning, which is what a list that is not the upstream's reports. */
	providerModelsWarning: string | null;
	/**
	 * The custom node's own read, which the details card makes beside the provider read (§7.4). It is served
	 * here rather than by the node stub because a node's screen needs this route and the model routes
	 * together, and two stubs would fight over `fetch`: the node stub's own tests keep the writes.
	 */
	providerNode: StubModel | null;
	/**
	 * The settings document the Connections section reads for its rotation switch (§7.14). It is served here
	 * because the section renders on the same screen as the model routes, and a second stub would fight over
	 * `fetch`.
	 */
	settings: StubModel;
	/** The bodies `PATCH /settings` received, which is where the whole override map shows up. */
	settingsPatches: StubModel[];
	/** The settings write's own status, for a test that needs the gateway to refuse a rotation change. */
	settingsWriteStatus: number;
	/** The settings read's own status, for a test that needs the reasoning picker's read to fail. */
	settingsReadStatus: number;
	/**
	 * The provider detail's `thinking_levels` (SPEC-API §7.14), which is what renders the reasoning
	 * picker. `null` is the absent field, which is what a provider whose models do not reason answers.
	 */
	thinkingLevels: string[] | null;
	/**
	 * The proxy pool rows the detail screen's proxy card reads for its pool select (§7.11). Served here
	 * because the card renders on the same screen as the model routes, and a second stub would fight
	 * over `fetch`.
	 */
	proxies: StubModel[];
	/** The pool list's own status, for a test that needs the card to survive a pool read failure. */
	proxyReadStatus: number;
};
