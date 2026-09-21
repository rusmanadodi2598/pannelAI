// The shared refresh control (docs/SPEC-UI/001-SPEC-UI.md §8.6.2).
//
// Two things are worth testing here rather than on a screen: that a click actually asks the screen to read
// again, and that a second click while that read is in flight is dropped instead of starting a second read
// (and without disabling the control, which is the rule the quota screen states).

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import RefreshControl from '../../src/lib/components/RefreshControl.svelte';

describe('RefreshControl', () => {
	afterEach(cleanup);

	it('names itself the way §8.6.2 does, and asks the screen to read again on a click', async () => {
		const onrefresh = vi.fn();
		render(RefreshControl, { props: { onrefresh } });

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		expect(onrefresh).toHaveBeenCalledTimes(1);
	});

	it('says what it is doing while the read is in flight, and stays pressable', async () => {
		let release: () => void = () => {};
		const pending = new Promise<void>((resolve) => {
			release = resolve;
		});
		const onrefresh = vi.fn(() => pending);
		render(RefreshControl, { props: { onrefresh } });

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		const button = screen.getByRole('button', { name: 'Refreshing' }) as HTMLButtonElement;
		expect(button.disabled).toBe(false);

		release();
		await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh now' })).toBeTruthy());
	});

	it('drops a second click while the first read is still running, rather than reading twice', async () => {
		let release: () => void = () => {};
		const pending = new Promise<void>((resolve) => {
			release = resolve;
		});
		const onrefresh = vi.fn(() => pending);
		render(RefreshControl, { props: { onrefresh } });

		const button = screen.getByRole('button', { name: 'Refresh now' });
		await fireEvent.click(button);
		await fireEvent.click(button);

		expect(onrefresh).toHaveBeenCalledTimes(1);

		release();
		await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh now' })).toBeTruthy());
		// Once the read lands, the control asks again, so the drop was a guard and not a dead control.
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
		expect(onrefresh).toHaveBeenCalledTimes(2);
	});
});
