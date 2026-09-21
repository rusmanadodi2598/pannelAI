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
		// The value slot before the gateway has answered. "unknown" is a claim about a read that finished,
		// so it may not stand in for one that has not.
		reading: 'Reading',
		// Two different facts get two sentences: a read that failed names a request, and a value that is not
		// a release number names the value itself.
		unreadable: 'Could not read the running version from the gateway.',
		notComparable: (value: string) =>
			`The gateway reports its version as ${value}, which is not a release number, so no release is marked.`
	},
	status: {
		upToDate: 'This build is the newest release listed.',
		behind: (count: number) => `${count} release${count === 1 ? '' : 's'} newer than this build.`
	},
	marker: {
		running: 'Running',
		newer: 'Newer',
		older: 'Installed',
		unknown: ''
	},
	source: {
		readFrom: (path: string) => `Read from GET ${path}.`,
		releases: (count: number) => `${count} release${count === 1 ? '' : 's'}`
	},
	empty: {
		title: 'The gateway serves no release notes',
		description: (path: string) =>
			`GET ${path} answered with an empty list. The notes travel inside the binary, so this is the gateway reporting its own history as empty rather than a source the panel is missing.`
	},
	error: {
		title: 'Release notes could not be read',
		retry: 'Try again'
	},
	loading: 'Loading release notes'
} as const;
