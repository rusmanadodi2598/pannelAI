export const SIDEBAR_COOKIE_NAME = "sidebar_state";
export const SIDEBAR_COOKIE_MAX_AGE = 60 * 60 * 24 * 7;
// 16.5rem is 264px at the default root size, the expanded width DESIGN.md §8 specifies. Upstream ships
// 16rem (256px); the panel follows the design direction, and tests/navigation/sidebar-metrics.test.ts
// asserts the px value so the two cannot drift.
export const SIDEBAR_WIDTH = "16.5rem";
export const SIDEBAR_WIDTH_MOBILE = "18rem";
export const SIDEBAR_WIDTH_ICON = "4rem";
export const SIDEBAR_KEYBOARD_SHORTCUT = "b";
