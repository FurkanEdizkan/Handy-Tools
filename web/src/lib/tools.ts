/**
 * The canonical tool catalog for the web UI.
 *
 * `defaultTools` pins the catalog used by `web/src/lib/pages/Home.svelte`
 * (the home grid) and `web/src/lib/routes.ts` (the sidebar). Keep all three
 * in sync when either changes.
 */

export interface ToolDef {
  /** Stable id — also the `:id` route param at `/tool/:id`. */
  id: string;
  /** Single-glyph icon, mirrored from the TUI catalog. */
  glyph: string;
  /** Short human title. */
  label: string;
  /** One-line description of what the tool does. */
  desc: string;
}

export const defaultTools: ToolDef[] = [
  {
    id: 'convert-image',
    glyph: '◇',
    label: 'Convert images',
    desc: 'PNG · JPEG · WebP · GIF · BMP · TIFF',
  },
  {
    id: 'zip-pack',
    glyph: '▢',
    label: 'Pack into archive',
    desc: 'zip · tar.gz · tar.bz2 · zstd · 7z',
  },
  {
    id: 'archive-extract',
    glyph: '◰',
    label: 'Extract archive',
    desc: 'zip · 7z · rar · tar · gz · bz2 · zst',
  },
  {
    id: 'pdf',
    glyph: '◫',
    label: 'PDF utilities',
    desc: 'merge · split · pages → image · text',
  },
  {
    id: 'diff-tree',
    glyph: '⇆',
    label: 'Diff two folders',
    desc: 'added · removed · changed (mtime or hash)',
  },
  {
    id: 'rename',
    glyph: '✎',
    label: 'Batch rename',
    desc: 'regex pattern → replacement, dry-run first',
  },
  {
    id: 'doctor',
    glyph: '◊',
    label: 'Doctor',
    desc: 'check optional system tools',
  },
];

/** toolById resolves a catalog entry by its id, or undefined if unknown. */
export function toolById(id: string): ToolDef | undefined {
  return defaultTools.find((t) => t.id === id);
}
