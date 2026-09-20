// Media provider contracts for the v1 surface in docs/SPEC-API/001-SPEC-API.md §7.10.
//
// The panel's kind vocabulary and the API's differ in exactly one place: the panel says `web` where
// the API says `search` (SPEC-UI §6.8). The mapping is total in both directions here, because the
// slug travels to the API as a body field and the API's kind travels back into a label, and a
// one-way map would let `/media-providers/web` request `?kind=web`, which the API refuses.
//
// A list row is a flat object: `MediaProviderResponse` embeds `MediaKindBlock` on the Go side, so
// the row schema extends the block schema rather than restating its fields. A field added to the
// block cannot then go missing from the row.
//
// `base_url_source` and `default_model_source` each report their own origin, and that is not
// decoration: it is what makes the self-hosted rule decidable. See `mediaBaseUrlRefusal`.

import { z } from 'zod';
import { nullableList } from './primitives';

// The panel's six kinds, in the order the sidebar lists them (SPEC-UI §6.8).
export const MEDIA_KINDS = ['embedding', 'image', 'video', 'tts', 'stt', 'web'] as const;
export type MediaKind = (typeof MEDIA_KINDS)[number];

export const MEDIA_KIND_LABELS: Record<MediaKind, string> = {
	embedding: 'Embedding',
	image: 'Image',
	video: 'Video',
	tts: 'TTS',
	stt: 'STT',
	web: 'Web Search'
};

// The API's six, from `domain.MediaKind` in app-serv. A closed set on the wire, so an unknown member
// is a contract change rather than a value to render.
export const MEDIA_API_KINDS = ['embedding', 'image', 'video', 'tts', 'stt', 'search'] as const;
export type MediaApiKind = (typeof MEDIA_API_KINDS)[number];

export const schemaMediaApiKind = z.enum(MEDIA_API_KINDS, {
	message: 'The API does not offer that media kind.'
});

const PANEL_TO_API: Record<MediaKind, MediaApiKind> = {
	embedding: 'embedding',
	image: 'image',
	video: 'video',
	tts: 'tts',
	stt: 'stt',
	web: 'search'
};

const API_TO_PANEL: Record<MediaApiKind, MediaKind> = {
	embedding: 'embedding',
	image: 'image',
	video: 'video',
	tts: 'tts',
	stt: 'stt',
	search: 'web'
};

/** The kind the API knows, for a kind the panel names. */
export function apiKindFor(kind: MediaKind): MediaApiKind {
	return PANEL_TO_API[kind];
}

/** The kind the panel names, for a kind the API sends. */
export function panelKindFor(kind: MediaApiKind): MediaKind {
	return API_TO_PANEL[kind];
}

/**
 * True when `value` is one of the panel's six kinds.
 *
 * Deliberately not case-folding: the panel's own links are lowercase, so an uppercase segment is a
 * hand-edited URL and saying so is more useful than silently accepting a spelling nothing produces.
 */
export function isMediaKind(value: string): value is MediaKind {
	return (MEDIA_KINDS as readonly string[]).includes(value);
}

export function mediaKindLabel(kind: MediaKind): string {
	return MEDIA_KIND_LABELS[kind];
}

/** Where a resolved value came from. Both sources report their own origin (§7.10). */
export const MEDIA_SOURCES = ['registry', 'override'] as const;
export type MediaSource = (typeof MEDIA_SOURCES)[number];

export const schemaMediaModel = z.object({
	id: z.string().min(1),
	name: z.string().optional(),
	// `omitempty` on the wire, and 0 for a model that has no dimension count, so an absent value must
	// not be read as zero.
	dimensions: z.number().int().nonnegative().optional()
});

export type MediaModel = z.infer<typeof schemaMediaModel>;

export const schemaMediaKindBlock = z.object({
	kind: schemaMediaApiKind,
	base_url: z.string(),
	base_url_source: z.enum(MEDIA_SOURCES, {
		message: 'The API reported an unknown base URL source.'
	}),
	default_model: z.string(),
	default_model_source: z.enum(MEDIA_SOURCES, {
		message: 'The API reported an unknown default model source.'
	}),
	endpoint_count: z.number().int().nonnegative(),
	models: nullableList(schemaMediaModel)
});

export type MediaKindBlock = z.infer<typeof schemaMediaKindBlock>;

export const schemaMediaProvider = schemaMediaKindBlock.extend({
	provider_id: z.string().min(1),
	provider_name: z.string().min(1)
});

export type MediaProvider = z.infer<typeof schemaMediaProvider>;

// §7.10 lists media providers without pagination: a deployment configures a handful of them.
export const schemaMediaProviderList = z.object({
	data: nullableList(schemaMediaProvider)
});

export type MediaProviderList = z.infer<typeof schemaMediaProviderList>;

// The detail answer carries every kind one provider offers. This screen reads one kind at a time and
// does not call it yet; the contract lives here so the shape has one home when a detail view needs it.
export const schemaMediaProviderDetail = z.object({
	provider_id: z.string(),
	provider_name: z.string(),
	media: nullableList(schemaMediaKindBlock)
});

export type MediaProviderDetail = z.infer<typeof schemaMediaProviderDetail>;

/**
 * The refusal the API would answer with, when the panel can prove it would.
 *
 * `service/media_provider.go` refuses a save that leaves the provider with no base URL from either
 * source, and its sentence is returned verbatim so the operator reads the same words whichever side
 * caught it. The panel can only prove that outcome in one of the three cases:
 *
 *   - source `registry`, stored value empty: the registry declares none, so an empty field means no
 *     base URL at all. This is the case the panel blocks.
 *   - source `registry`, stored value present: clearing the field falls back to a real value.
 *   - source `override`: the registry's value is only read when the override is empty, so it is not
 *     on the wire and the panel cannot know whether clearing is safe. The server decides.
 *
 * Returns null when there is nothing to refuse.
 */
export function mediaBaseUrlRefusal(input: {
	providerId: string;
	block: MediaKindBlock;
	baseUrl: string;
}): string | null {
	if (input.baseUrl.trim() !== '') return null;
	if (input.block.base_url_source !== 'registry') return null;
	if (input.block.base_url !== '') return null;

	return `provider ${input.providerId} has no ${input.block.kind} base_url; set one`;
}
