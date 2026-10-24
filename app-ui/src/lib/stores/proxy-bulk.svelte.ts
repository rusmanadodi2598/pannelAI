// The proxy pool's selection and the writes that act on it (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// The page owns the list; this owns what the operator selected and the three writes that can happen to a
// selection. They share one error slot because one refusal is shown in one place, whether it came from a
// row switch or from a bulk erase.

import { bulkProxyDelete, bulkProxyPreview, bulkProxyStatus } from '$lib/api/bulk-mutation';
import { updateProxy } from '$lib/api/proxies';
import type { ApiError } from '$lib/api/errors';
import { emptySelection, toggleAll, toggleRow, type Selection } from '$lib/utils/selection';
import type { Proxy, ProxyForm } from '$lib/schemas/proxy';

type Options = {
	/** Every id the list currently reports, which is what a row toggle reconciles against. */
	ids: () => string[];
	/** The candidate count the server reports, which is what Select-All claims. */
	total: () => number;
	/** Re-read the pool after a write, so a row shows what the gateway recorded (§8.6.3). */
	onapplied: () => Promise<void>;
	/** The erase's answer, stated in rows and in the pins that went with them. */
	onremoved: (message: string) => void;
};

export function createProxyBulkActions(options: Options) {
	let selected = $state<Selection>(emptySelection);
	let error = $state<ApiError | null>(null);
	let busy = $state(false);

	const count = $derived(selected.all ? options.total() : selected.ids.length);
	const scope = $derived(
		selected.all
			? `all ${options.total()} candidates on the server`
			: `${selected.ids.length} named by you`
	);

	function toggle(id: string, checked: boolean): void {
		selected = toggleRow(selected, id, checked, options.ids());
	}

	function toggleEvery(checked: boolean): void {
		selected = toggleAll(checked);
	}

	function clear(): void {
		selected = emptySelection;
	}

	/** The selector the bulk routes need: the whole set when Select-All is on, the named ids otherwise. */
	function selector(): { ids?: string[]; all?: true } {
		return selected.all ? { all: true } : { ids: selected.ids };
	}

	function formFor(proxy: Proxy, enabled: boolean): ProxyForm {
		return {
			label: proxy.label,
			protocol: proxy.protocol,
			host: proxy.host,
			port: proxy.port,
			username: proxy.username,
			// An empty password keeps the stored secret: the wire carries `has_password` and never
			// the value, so a switch cannot restate a credential it was not given.
			password: '',
			enabled
		};
	}

	async function setRowEnabled(proxy: Proxy, enabled: boolean): Promise<void> {
		const result = await updateProxy(proxy.id, formFor(proxy, enabled));
		error = result.ok ? null : result.error;
		if (result.ok) await options.onapplied();
	}

	async function setStatus(enabled: boolean): Promise<void> {
		if (count === 0) return;
		busy = true;
		const result = await bulkProxyStatus(selector(), enabled, count);
		busy = false;
		error = result.ok ? null : result.error;
		if (result.ok) await options.onapplied();
	}

	// The erase is confirmed against the number storage reports, not a number this page guesses: a
	// candidate is pinned by an id string, so only the server knows which connections lose their egress
	// path. Ask, show the answer, then erase with that figure echoed back.
	async function erase(): Promise<void> {
		if (count === 0) return;
		busy = true;
		const preview = await bulkProxyPreview(selector(), count);
		if (!preview.ok) {
			busy = false;
			error = preview.error;
			return;
		}

		const result = await bulkProxyDelete(selector(), count, preview.data.pins.endpoints);
		busy = false;
		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		options.onremoved(
			`Removed ${result.data.changed} candidates. ${result.data.pins.endpoints} endpoint pin(s) across ${result.data.pins.providers.length} provider(s) went with them.`
		);
		clear();
		await options.onapplied();
	}

	return {
		get selected() {
			return selected;
		},
		get error() {
			return error;
		},
		get busy() {
			return busy;
		},
		get count() {
			return count;
		},
		get scope() {
			return scope;
		},
		toggle,
		toggleEvery,
		clear,
		setRowEnabled,
		setStatus,
		erase
	};
}

export type ProxyBulkActions = ReturnType<typeof createProxyBulkActions>;
