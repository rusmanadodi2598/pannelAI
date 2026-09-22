// Beam rows for the live drawing (src/lib/components/UsageTopologyDrawing.svelte, draft 015 F1).
//
// A routing edge is the reference fork's beam: a wide halo, a dashed plasma and a dashed core, with six
// orbs and five sparks travelling along the same line. These rows are about which of those parts are
// rendered and what numbers they carry, rather than about the pixels they end up drawing, which the
// browser pass measures. The gates that switch the beam off live next door in
// `usage-topology-motion.test.ts`.

import { cleanup } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { beamAt, draw, entry, linesAt } from '../support/topology-harness';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
});

describe('UsageTopology beam', () => {
	it('draws the routing edge as the reference beam, and draws no other edge that way', () => {
		const container = draw({ active: [entry('openai')] });

		expect(beamAt(container, 'OpenAI', 'halo')).toHaveLength(1);
		expect(beamAt(container, 'OpenAI', 'plasma')).toHaveLength(1);
		expect(beamAt(container, 'OpenAI', 'core')).toHaveLength(1);
		expect(linesAt(container, 'Anthropic')).toHaveLength(1);
		expect(beamAt(container, 'Anthropic', 'core')).toHaveLength(0);
	});

	it('carries the two dash patterns the drawing declares, in the path own units', () => {
		const container = draw({ active: [entry('openai')] });

		expect(beamAt(container, 'OpenAI', 'core')[0].getAttribute('style')).toContain(
			'stroke-dasharray: 6 4'
		);
		expect(beamAt(container, 'OpenAI', 'plasma')[0].getAttribute('style')).toContain(
			'stroke-dasharray: 14 11'
		);
		expect(beamAt(container, 'OpenAI', 'halo')[0].getAttribute('style')).toBeNull();
	});

	it('runs six orbs and five sparks along that edge, and none along an idle one', () => {
		const container = draw({ active: [entry('openai')] });

		expect(beamAt(container, 'OpenAI', 'orb')).toHaveLength(6);
		expect(beamAt(container, 'OpenAI', 'spark')).toHaveLength(5);
		expect(beamAt(container, 'Anthropic', 'orb')).toHaveLength(0);
	});

	it('gives every particle the width, speed and phase the reference gives it', () => {
		const container = draw({ active: [entry('openai')] });
		const orbs = beamAt(container, 'OpenAI', 'orb').map((line) => line.getAttribute('style') ?? '');
		const sparks = beamAt(container, 'OpenAI', 'spark').map(
			(line) => line.getAttribute('style') ?? ''
		);

		expect(orbs[0]).toContain('stroke-width: 8');
		expect(orbs[1]).toContain('stroke-width: 5');
		expect(orbs[0]).toContain('animation-duration: 0.4s');
		expect(orbs[0]).toContain('animation-delay: 0s');
		expect(orbs[1]).toContain('animation-delay: -0.09s');
		expect(orbs[5]).toContain('animation-duration: 0.8s');
		expect(sparks[0]).toContain('animation-duration: 0.28s, 0.35s');
		expect(sparks[2]).toContain('animation-duration: 0.38s, 0.55s');
	});

	it('gives every particle one dot and a bare path, so a line is a traveller, not a row of dots', () => {
		// Six orbs each painting six dots made the edge a chain in the browser (draft 015 F2).
		const container = draw({ active: [entry('openai')] });
		const particles = [
			...beamAt(container, 'OpenAI', 'orb'),
			...beamAt(container, 'OpenAI', 'spark')
		];

		for (const line of particles) {
			expect(line.getAttribute('style')).toContain('stroke-dasharray: 0.01 99.99');
		}
	});

	it('tones every third particle away from the status colour, so a dot is visible on the beam', () => {
		const container = draw({ active: [entry('openai')] });
		const orbs = beamAt(container, 'OpenAI', 'orb');
		const sparks = beamAt(container, 'OpenAI', 'spark');

		expect(orbs[0].classList.contains('stroke-[var(--color-ok)]')).toBe(true);
		expect(orbs[2].classList.contains('stroke-[var(--color-text)]')).toBe(true);
		expect(orbs[3].classList.contains('stroke-[var(--color-ok)]')).toBe(true);
		expect(sparks.every((spark) => spark.classList.contains('stroke-[var(--color-ok)]'))).toBe(
			true
		);
	});

	it('holds every dot on the edge it belongs to, with the stroke width the stretch cannot change', () => {
		const container = draw({ active: [entry('openai')] });

		for (const line of [
			...beamAt(container, 'OpenAI', 'orb'),
			...beamAt(container, 'OpenAI', 'core')
		]) {
			expect(line.getAttribute('pathLength')).toBe('100');
			expect(line.getAttribute('vector-effect')).toBe('non-scaling-stroke');
			expect(line.getAttribute('x1')).toBe('50');
		}
	});
});
