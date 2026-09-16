// Theme state.
//
// Both modes are shipped and both are authored, so the toggle is real work rather than a class flip
// (docs/SPEC-UI/001-SPEC-UI.md §8.9). The system preference decides the first visit; an explicit
// choice is remembered afterwards.

export type ThemeName = 'light' | 'dark';

const STORAGE_KEY = 'pannelai.theme';

function createThemeStore() {
	let current = $state<ThemeName>('light');

	function apply(name: ThemeName): void {
		current = name;
		if (typeof document === 'undefined') return;
		document.documentElement.classList.toggle('dark', name === 'dark');
	}

	function init(): void {
		if (typeof window === 'undefined') return;

		const stored = window.localStorage.getItem(STORAGE_KEY);
		if (stored === 'light' || stored === 'dark') {
			apply(stored);
			return;
		}

		apply(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
	}

	function toggle(): void {
		const next: ThemeName = current === 'dark' ? 'light' : 'dark';
		apply(next);
		if (typeof window !== 'undefined') window.localStorage.setItem(STORAGE_KEY, next);
	}

	return {
		get name() {
			return current;
		},
		init,
		toggle
	};
}

export const theme = createThemeStore();
