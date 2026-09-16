// Boot banner for the panel process.
//
// Printed once before the server starts, so an operator can see the runtime version, the panel
// version, and where /api/v1 is pointing without reading a config file
// (docs/SPEC-UI/001-SPEC-UI.md §10.1.5). Loaded with `bun --preload`, which keeps the built server
// untouched.

const packageJson = (await Bun.file(new URL('../package.json', import.meta.url)).json()) as {
	version?: string;
};

const target =
	process.env.PANEL_API_TARGET ?? '(unset, requests to /api/v1 will return a config error)';

console.log(`pannelAI panel ${packageJson.version ?? 'unknown'} on Bun ${Bun.version}`);
console.log(`API target: ${target}`);
