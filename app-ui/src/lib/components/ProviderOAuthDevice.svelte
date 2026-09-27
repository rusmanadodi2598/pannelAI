<script lang="ts">
	// The device authorization round of the provider OAuth section
	// (docs/SPEC-API/001-SPEC-API.md §7.4, docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// A device flow has no browser callback, so the panel drives it by asking: start once, then poll until
	// the vendor grants the token or the round's own window closes. The gateway holds the PKCE verifier and
	// the machine id; the only thing this screen keeps is the device code, which is single-use.
	//
	// The verification URL is shown as a link the operator opens, never followed from here, and it stays
	// readable as text beside that link: the panel does not navigate to a third party on the operator's
	// behalf (the same rule the code flow's authorize link keeps, and the reason the reference's automatic
	// `window.open` was not ported).
	//
	// The loop is one timer, not an interval, so an in-flight ask can never stack a second one behind it,
	// and it stops on its own at the deadline the gateway stated. Stopping is also something the operator
	// can do, because a round left polling is a round still asking a vendor.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import { pollProviderOAuthDevice, startProviderOAuthDevice } from '$lib/api/providers';
	import {
		oauthDeviceConnected,
		type OAuthDevicePoll,
		type OAuthDeviceStart
	} from '$lib/schemas/oauth';

	let {
		providerId,
		onconnected
	}: {
		providerId: string;
		onconnected: () => void;
	} = $props();

	let round = $state<OAuthDeviceStart | null>(null);
	let starting = $state(false);
	let startError = $state<string | null>(null);
	let polling = $state(false);
	let pollError = $state<string | null>(null);
	let expired = $state(false);
	let connected = $state<OAuthDevicePoll | null>(null);

	let timer: ReturnType<typeof setTimeout> | null = null;
	let deadline = 0;

	function stop(): void {
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
		polling = false;
	}

	// Leaving the screen must not leave a timer asking the vendor about a round nobody is watching.
	$effect(() => () => stop());

	async function start(): Promise<void> {
		stop();
		starting = true;
		startError = null;
		pollError = null;
		expired = false;
		connected = null;

		const result = await startProviderOAuthDevice(providerId);
		starting = false;
		if (!result.ok) {
			round = null;
			startError = result.error.message;
			return;
		}

		round = result.data;
		polling = true;
		deadline = Date.now() + result.data.expires_in * 1000;
		schedule(result.data.interval_seconds * 1000);
	}

	function schedule(delayMs: number): void {
		if (timer !== null) clearTimeout(timer);
		timer = setTimeout(() => void ask(), delayMs);
	}

	async function ask(): Promise<void> {
		const current = round;
		if (current === null || !polling) return;
		if (Date.now() >= deadline) {
			stop();
			expired = true;
			return;
		}

		const result = await pollProviderOAuthDevice(providerId, current.device_code);
		// The operator stopped, or the section unmounted, while the ask was in flight.
		if (!polling) return;

		if (!result.ok) {
			stop();
			pollError = result.error.message;
			return;
		}
		if (oauthDeviceConnected(result.data)) {
			stop();
			connected = result.data;
			onconnected();
			return;
		}
		if (result.data.status !== 'pending') {
			stop();
			pollError = `The gateway answered “${result.data.status}”, which this screen cannot act on. Start another round.`;
			return;
		}
		schedule(current.interval_seconds * 1000);
	}
</script>

<div class="flex flex-col gap-3">
	<button
		type="button"
		class="min-h-11 self-start rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4 disabled:opacity-50"
		disabled={starting}
		onclick={() => void start()}
	>
		{starting
			? 'Starting'
			: connected
				? 'Connect another account'
				: 'Start the device authorization'}
	</button>

	{#if startError}
		<p class="text-sm text-[var(--color-danger)]" role="alert">{startError}</p>
	{/if}

	{#if round}
		<div
			class="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] p-3"
		>
			<h4 class="text-sm font-semibold">Device page</h4>
			<p class="text-sm text-[var(--color-text-muted)]">
				Open this address and approve the request there. The panel does not open it for you.
			</p>
			<p class="flex flex-wrap items-center gap-2">
				<!-- eslint-disable svelte/no-navigation-without-resolve -- the device page is the vendor's
					own absolute URL, never a route of this app, and the operator is the one who opens it -->
				<a
					href={round.verification_url}
					target="_blank"
					rel="noopener noreferrer"
					class="min-h-11 content-center break-all underline"
				>
					Open the device page
				</a>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
				<CopyButton value={round.verification_url} label="Copy the device page address" />
			</p>
			<p class="break-all font-mono text-xs text-[var(--color-text-muted)]">
				{round.verification_url}
			</p>

			<h4 class="pt-1 text-sm font-semibold">Your code</h4>
			<p class="flex flex-wrap items-center gap-2">
				<span class="font-mono text-lg" data-testid="device-user-code">{round.user_code}</span>
				<CopyButton value={round.user_code} label="Copy your code" />
			</p>
		</div>

		{#if polling}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				Waiting for the vendor to grant the request…
			</p>
			<button
				type="button"
				class="min-h-11 self-start rounded-[var(--radius-sm)] border border-[var(--color-border)] px-4"
				onclick={stop}
			>
				Stop waiting
			</button>
		{:else if expired}
			<p class="text-sm text-[var(--color-danger)]" role="alert">
				This round expired before the request was approved. Start another one.
			</p>
		{:else if pollError}
			<p class="text-sm text-[var(--color-danger)]" role="alert">{pollError}</p>
		{:else if connected}
			<p class="text-sm" role="status">
				Connected{connected.endpoint_id ? ` (${connected.endpoint_id})` : ''}.
				{connected.created
					? 'A new account is listed below.'
					: 'The known account has its new token.'}
			</p>
		{/if}
	{/if}
</div>
