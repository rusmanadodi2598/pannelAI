// Skill catalog schema tests (docs/SPEC-UI/001-SPEC-UI.md §6.10, SPEC-API §7.16).
//
// The catalog belongs to the gateway, so the schema is the panel's contract with it: what it must carry
// is strict, what it may add is tolerated (§7.4). The negative cases matter as much as the positive one,
// because a row the panel cannot read must fail loudly rather than render a blank card.

import { describe, expect, it } from 'vitest';
import { schemaSkill, schemaSkillCatalog } from '$lib/schemas/skill';
import { DEFAULT_ROWS, skillCatalog, skillRow } from '../support/skill-catalog';

describe('skill catalog schema', () => {
	it('parses the shape §7.16 serves, entry row first', () => {
		const parsed = schemaSkillCatalog.parse(skillCatalog());

		expect(parsed.data).toHaveLength(3);
		expect(parsed.data[0].id).toBe('pannelai');
		expect(parsed.data[0].entry).toBe(true);
		expect(parsed.data[0].endpoint).toBeNull();
		expect(parsed.data[1].endpoint).toBe('/chat/completions');
	});

	it('tolerates a field the panel does not read', () => {
		const rows = [{ ...DEFAULT_ROWS[0], icon: 'hub', phase: 'P4' }];

		expect(schemaSkillCatalog.parse(skillCatalog(rows)).data[0].id).toBe('pannelai');
	});

	it('keeps the entry flag as the gateway states it, rather than inferring it from a null endpoint', () => {
		const parsed = schemaSkill.parse({ ...DEFAULT_ROWS[1], endpoint: null });

		expect(parsed.entry).toBe(false);
		expect(parsed.endpoint).toBeNull();
	});

	it('refuses a row whose addresses are not absolute URLs', () => {
		const relative = { ...DEFAULT_ROWS[1], raw_url: 'skills/pannelai-chat/SKILL.md' };

		expect(schemaSkill.safeParse(relative).success).toBe(false);
	});

	it('refuses a row that is missing a field the screen renders', () => {
		const { description: _dropped, ...withoutDescription } = skillRow(
			'pannelai-stt',
			'Speech-to-Text',
			'/audio/transcriptions'
		);

		expect(schemaSkill.safeParse(withoutDescription).success).toBe(false);
	});

	it('refuses an answer that is not wrapped in data', () => {
		expect(schemaSkillCatalog.safeParse(DEFAULT_ROWS).success).toBe(false);
	});
});
