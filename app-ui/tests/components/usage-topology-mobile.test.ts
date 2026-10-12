// Usage topology mobile P0 Fase A (F28-A, F26, F23/F24).
//
// Owner decision: desktop is already good, nodes are solid, mobile only draws
// too small, so mobile gets a horizontal scroll region instead of a smaller
// drawing. Each table below pins a class contract, so a restyle that drops the
// mobile behavior fails here first. One `it` per finding holds its cases in an
// array, following TDD §2.5.

import { cleanup, render, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageTopology from '../../src/lib/components/UsageTopology.svelte';
import { beamAt, draw, entry } from '../support/topology-harness';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
});

describe('UsageTopology mobile P0 Fase A', () => {
	it('F28-A keeps the page from scrolling sideways: the scroll lives in the frame', () => {
		const container = draw({ active: [entry('openai')] });
		const figure = container.querySelector('figure') as HTMLElement;
		const scroller = figure.firstElementChild as HTMLElement;
		const inner = scroller.firstElementChild as HTMLElement;
		const box = container.querySelector('div[aria-hidden="true"]') as HTMLElement;

		const cases = [
			{
				name: 'scroller scrolls on overflow',
				pass: scroller.classList.contains('overflow-x-auto')
			},
			{ name: 'inner holds the desktop width', pass: inner.classList.contains('min-w-[520px]') },
			{ name: 'drawing still fills its box', pass: box.classList.contains('w-full') },
			{
				name: 'drawing still owns the container unit',
				pass: box.classList.contains('[container-type:inline-size]')
			},
			{ name: 'figure itself never scrolls', pass: !figure.classList.contains('overflow-x-auto') }
		];

		for (const item of cases) {
			expect(item.pass, item.name).toBe(true);
		}
	});

	it('F26 gives the frame mobile margins and padding, desktop keeps its own', () => {
		const container = draw();
		const figure = container.querySelector('figure') as HTMLElement;
		const scroller = figure.firstElementChild as HTMLElement;

		const cases = [
			{ name: 'figure pads tight on mobile', pass: figure.classList.contains('p-2') },
			{ name: 'figure restores padding past mobile', pass: figure.classList.contains('sm:p-4') },
			{ name: 'no bare desktop padding on the figure', pass: !figure.classList.contains('p-4') },
			{ name: 'scroller margins tight on mobile', pass: scroller.classList.contains('mx-2') },
			{
				name: 'scroller restores margins past mobile',
				pass: scroller.classList.contains('sm:mx-16')
			},
			{
				name: 'no bare desktop margin on the scroller',
				pass: !scroller.classList.contains('mx-12')
			}
		];

		for (const item of cases) {
			expect(item.pass, item.name).toBe(true);
		}
	});

	it('F23/F24 floors every scaled metric, so a narrow box never draws tiny text', () => {
		const container = draw({ active: [entry('openai')] });
		const gateway = within(container).getByText('Gateway').parentElement as HTMLElement;
		const terminal = within(container).getByText('Client') as HTMLElement;
		const node = within(container).getByText('OpenAI').parentElement as HTMLElement;
		const dot = node.querySelector('span.relative') as HTMLElement;
		const logo = container.querySelector('img') as HTMLElement;
		const badge = within(gateway).getByText('1');

		const cases = [
			{
				name: 'gateway label floors at 12px',
				pass: gateway.classList.contains('[font-size:max(12px,calc(14px*var(--u)))]')
			},
			{
				name: 'terminal label floors at 11px',
				pass: terminal.classList.contains('[font-size:max(11px,calc(13px*var(--u)))]')
			},
			{
				name: 'node label floors at 12px',
				pass: node.classList.contains('[font-size:max(12px,calc(14px*var(--u)))]')
			},
			{ name: 'state dot never shrinks past 8px', pass: dot.classList.contains('size-[8px]') },
			{
				name: 'gateway mark never shrinks past 14px',
				pass: logo.classList.contains('size-[max(14px,calc(16px*var(--u)))]')
			},
			{
				name: 'count badge floors its width at 20px',
				pass: badge.classList.contains('min-w-[max(20px,calc(20px*var(--u)))]')
			},
			{
				name: 'count badge floors its text at 11px',
				pass: badge.classList.contains('text-[max(11px,0.857em)]')
			}
		];

		for (const item of cases) {
			expect(item.pass, item.name).toBe(true);
		}
	});

	it('F23/F24 floors the beam strokes, so a narrow box keeps a visible path', () => {
		const container = draw({ active: [entry('openai')] });

		const cases = [
			{
				name: 'core floors at 1.5px',
				pass: beamAt(container, 'OpenAI', 'core')[0].classList.contains(
					'[stroke-width:max(1.5px,calc(2.2px*var(--u)))]'
				)
			},
			{
				name: 'plasma floors at 2px',
				pass: beamAt(container, 'OpenAI', 'plasma')[0].classList.contains(
					'[stroke-width:max(2px,calc(5px*var(--u)))]'
				)
			}
		];

		for (const item of cases) {
			expect(item.pass, item.name).toBe(true);
		}
	});

	it('empty state renders no scroll region at all', () => {
		const { container } = render(UsageTopology, {
			props: { providers: [], combos: [], active: [], last: '', error: '', live: true }
		});
		const figure = container.querySelector('figure') as HTMLElement;

		const cases = [
			{ name: 'figure still pads tight on mobile', pass: figure.classList.contains('p-2') },
			{ name: 'no scroller without a drawing', pass: !figure.querySelector('.overflow-x-auto') },
			{ name: 'no min-width without a drawing', pass: !figure.querySelector('.min-w-\\[520px\\]') }
		];

		for (const item of cases) {
			expect(item.pass, item.name).toBe(true);
		}
	});
});
