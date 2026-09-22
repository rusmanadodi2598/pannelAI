// Boot banner for the panel process.
//
// Printed once before the server starts, so an operator can see the runtime version, the panel
// version, the runtime environment, the port it binds, and where /api/v1 is pointing without reading
// a config file (docs/SPEC-UI/001-SPEC-UI.md §10.1 item 5). Loaded with `bun --preload`, which keeps
// the built server untouched.
//
// This is also where APP_PORT becomes the port the server binds: adapter-node reads `PORT` and the
// env template calls it `APP_PORT` (src/lib/schemas/env.ts). An explicit PORT still wins, so an
// operator who sets both is not silently overridden. A value the schema refuses stops the process
// here, before the server module loads, rather than leaving a panel on a port nobody asked for.

import { EnvError, parseRuntime, type PanelRuntime } from '../src/lib/schemas/env';

const packageJson = (await Bun.file(new URL('../package.json', import.meta.url)).json()) as {
	version?: string;
};

let runtime: PanelRuntime;
try {
	runtime = parseRuntime(process.env);
} catch (error) {
	if (!(error instanceof EnvError)) throw error;
	console.error(`pannelAI panel: ${error.message}`);
	process.exit(1);
}

const port = process.env.PORT ?? String(runtime.APP_PORT);
process.env.PORT = port;

const target =
	process.env.PANEL_API_TARGET ?? '(unset, requests to /api/v1 will return a config error)';

console.log(
	`pannelAI panel ${packageJson.version ?? 'unknown'} on Bun ${Bun.version} (${runtime.APP_ENV})`
);
console.log(`Listening on port ${port}`);
console.log(`API target: ${target}`);
