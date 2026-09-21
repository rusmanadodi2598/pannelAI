// The gateway base address the panel server hands out (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// The panel reads it with a strict schema like any other served value, and the check is not decoration:
// the address is rendered into a client's configuration and into a copyable command, so a value that is
// not an absolute http(s) URL has to be refused at the boundary rather than pasted into a CLI.

import { z } from 'zod';
import { absoluteUrl } from './primitives';

export const schemaApiBase = z.object({ base_url: absoluteUrl });

export type ApiBase = z.infer<typeof schemaApiBase>;
