// Release note copy for the changelog screen.
//
// One module for the panel's own prose on this screen (SPEC-UI §8.10.1), so a wording change is one edit
// and no string is composed inside a component. English only, no em dash (R-02), no marketing vocabulary
// (R-16), and no figure the data source did not provide (R-17).

export const CHANGELOG_COPY = {
	title: 'Changelog',
	// What the screen is for, stated as the question it answers rather than as a description of itself.
	subtitle: 'What changed in the gateway, and whether this build is behind.',
	running: {
		label: 'Running version',
		unknown: 'Could not read the running version from the gateway.'
	},
	status: {
		upToDate: 'This build is the newest release listed.',
		behind: (count: number) => `${count} release${count === 1 ? '' : 's'} newer than this build.`,
		unknown:
			'The running version could not be read, so no release is marked as newer or older than this build.'
	},
	marker: {
		running: 'Running',
		newer: 'Newer',
		older: 'Installed',
		unknown: ''
	},
	category: {
		feature: 'Feature',
		fix: 'Fix',
		change: 'Change',
		security: 'Security',
		internal: 'Internal'
	},
	empty: {
		title: 'No release notes yet',
		description:
			'The gateway reports its version, but no release notes are available to the panel. Notes appear here once a source is configured.'
	},
	error: {
		title: 'Release notes could not be loaded',
		retry: 'Try again'
	},
	loading: 'Loading release notes',
	dateUnknown: 'Release date not available',
	entriesCounted: (count: number) => `${count} entry${count === 1 ? '' : 's'}`
} as const;
