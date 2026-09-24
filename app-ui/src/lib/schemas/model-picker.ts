// The reference picker's data model (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The reference never offers its whole registry. `ModelSelectModal.js:216-219` builds the set it shows
// from the providers that have a connection ("Only show connected providers"), and this module is that
// rule for this panel: the catalog is the whole registry, and a provider that was never configured must
// not be offered as if it could route.
//
// The panel's own word for "configured" is the provider row's `endpoint_count`; `statusSummaryText`
// already reads a zero as "No endpoint configured". The filter is exactly `endpoint_count > 0` and not
// the reference's "or no_auth", because this gateway selects an endpoint row for every provider before
// it routes, the no_auth ones included (draft 024 §3.7 F7), so a no_auth provider with no row answers
// `NO_PROVIDER_AVAILABLE` like any other. Offering it would be the same defect this pass removes.
//
// A provider the catalog carries but the list does not (the registry's hidden entries, which the list
// route excludes because their id is reachable only through another entry's alias) is dropped rather
// than guessed at: it cannot be configured through the panel, so it cannot be proven active. The combo
// editor's own text inputs keep such a ref typeable either way.
//
// Two further rules come from the same question ("could a ref this picker offers ever answer?"), and both
// read fields the panel already has: a provider whose chat path needs a connector is dropped
// (`routability`), and a media model is dropped (`kind`). Both are refs the write path refuses with a
// concrete reason (draft 024 F4), so offering them would be a save that cannot succeed.

import type { CatalogModel } from './model';
import { catalogModelLabel } from './model';
import type { Provider } from './provider';

// One chip in the picker. `value` is the ref that gets stored; `label` is what the chip reads.
export type PickerOption = {
	value: string;
	label: string;
	/** A connected provider with no catalog rows yet: add it, then edit the model id in the editor. */
	placeholder?: boolean;
};

// One heading and its chips. `key` is stable (`combos`, or the provider id) so the dialog can key rows.
export type PickerSection = {
	key: string;
	label: string;
	options: PickerOption[];
};

/** The ids whose provider has at least one endpoint, which is this panel's "active". */
export function activeProviderIds(providers: Provider[]): Set<string> {
	return new Set(
		providers.filter((provider) => provider.endpoint_count > 0).map((provider) => provider.id)
	);
}

// The kinds that serve chat completions, which is the registry's own rule (`registry/types.go:131-140`):
// a model with no kind is chat, and so is one declared `llm` or `chat`. Every other kind is media, so the
// write path refuses it (draft 024 F4) and the picker must not offer it.
function isChatModel(model: CatalogModel): boolean {
	const kind = model.kind ?? '';
	return kind === '' || kind === 'llm' || kind === 'chat';
}

// The sections the picker renders: the combos first, as the reference puts them, then one section per
// active provider in the order the list route returned (priority, then id), each sorted by label so two
// identical reads produce identical screens.
//
// `placeholders` is false when the catalog was read for one capability. The reference's capability filter
// keeps only the models whose caps include it (`ModelSelectModal.js:448-451`), which drops its placeholder
// chip as a side effect: a placeholder's ref has no reported caps. The rule is kept as a rule rather than
// as that accident, because a dashed entry in a "vision models" list would offer a ref whose capability
// nothing has confirmed.
export function pickerSections({
	catalog,
	providers,
	combos,
	placeholders = true
}: {
	catalog: CatalogModel[];
	providers: Provider[];
	combos: string[];
	placeholders?: boolean;
}): PickerSection[] {
	const active = activeProviderIds(providers);
	const sections: PickerSection[] = [];

	const comboNames = [...new Set(combos)].sort();
	if (comboNames.length > 0) {
		sections.push({
			key: 'combos',
			label: 'Combos',
			options: comboNames.map((name) => ({ value: name, label: name }))
		});
	}

	for (const provider of providers) {
		if (!active.has(provider.id)) continue;

		// A provider whose chat path needs a connector can be configured but never answers chat
		// (`RoutableNeedsConnector`; draft 024 F4 refuses its refs at write time), so it is not offered.
		if (provider.routability !== 'native') continue;

		const options = catalog
			.filter((model) => model.provider_id === provider.id && isChatModel(model))
			.map((model) => ({ value: model.id, label: catalogModelLabel(model) }))
			.sort((left, right) => left.label.localeCompare(right.label));

		if (options.length === 0 && !placeholders) continue;

		// A connected provider with no rows of its own is still pickable, which is the reference's own
		// shape (`ModelSelectModal.js:335-339`): one dashed placeholder that pre-fills a ref the operator
		// then edits. Without it, a node whose upstream has not answered yet would vanish from the picker.
		sections.push({
			key: provider.id,
			label: provider.name,
			options:
				options.length > 0
					? options
					: [
							{
								value: `${provider.id}/model-id`,
								label: `${provider.id}/model-id`,
								placeholder: true
							}
						]
		});
	}

	return sections;
}

// The dialog's search. A section whose heading matches keeps all of its options, which is the
// reference's own behaviour (`ModelSelectModal.js:453-459`): searching a provider name is a way to see
// everything that provider offers.
export function filterPickerSections(sections: PickerSection[], query: string): PickerSection[] {
	const needle = query.trim().toLowerCase();
	if (needle === '') return sections;

	const matched: PickerSection[] = [];
	for (const section of sections) {
		if (section.label.toLowerCase().includes(needle)) {
			matched.push(section);
			continue;
		}
		const options = section.options.filter(
			(option) =>
				option.label.toLowerCase().includes(needle) || option.value.toLowerCase().includes(needle)
		);
		if (options.length > 0) matched.push({ ...section, options });
	}
	return matched;
}

/** Every option in the sections, flattened, which is what the dialog counts and tests read. */
export function pickerOptions(sections: PickerSection[]): PickerOption[] {
	return sections.flatMap((section) => section.options);
}
