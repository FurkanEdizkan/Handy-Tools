import type { Component } from 'svelte';
import Home from './pages/Home.svelte';
import ToolPage from './pages/ToolPage.svelte';
import Doctor from './pages/Doctor.svelte';
import Jobs from './pages/Jobs.svelte';
import Settings from './pages/Settings.svelte';
import DevComponents from './pages/DevComponents.svelte';
import NotFound from './pages/NotFound.svelte';

export const routes: Record<string, Component> = {
  '/': Home,
  '/tool/:id': ToolPage,
  '/doctor': Doctor,
  '/jobs': Jobs,
  '/settings': Settings,
  // Dev-only component gallery (issue #70) — intentionally not in `navItems`.
  '/dev/components': DevComponents,
  '*': NotFound,
};

export type NavSection = 'tools' | 'system';

export type NavItem = {
  href: string;
  label: string;
  glyph: string;
  section: NavSection;
  /** Matches the current router path for the active highlight. */
  matches: (path: string) => boolean;
};

/**
 * Sidebar navigation, grouped into Tools and System sections to mirror the
 * htools-gui design. The tool entries deep-link to `/tool/:id`.
 */
export const navItems: NavItem[] = [
  { href: '#/', label: 'Home', glyph: '⌂', section: 'tools', matches: (p) => p === '/' || p === '' },
  {
    href: '#/tool/convert-image',
    label: 'Convert images',
    glyph: '◇',
    section: 'tools',
    matches: (p) => p === '/tool/convert-image',
  },
  {
    href: '#/tool/zip-pack',
    label: 'Pack into archive',
    glyph: '▢',
    section: 'tools',
    matches: (p) => p === '/tool/zip-pack',
  },
  {
    href: '#/tool/archive-extract',
    label: 'Extract archive',
    glyph: '◰',
    section: 'tools',
    matches: (p) => p === '/tool/archive-extract',
  },
  {
    href: '#/tool/pdf',
    label: 'PDF utilities',
    glyph: '◫',
    section: 'tools',
    matches: (p) => p === '/tool/pdf',
  },
  {
    href: '#/tool/hash',
    label: 'Hash files',
    glyph: '#',
    section: 'tools',
    matches: (p) => p === '/tool/hash',
  },
  {
    href: '#/tool/diff-tree',
    label: 'Diff two folders',
    glyph: '⇆',
    section: 'tools',
    matches: (p) => p === '/tool/diff-tree',
  },
  {
    href: '#/tool/rename',
    label: 'Batch rename',
    glyph: '✎',
    section: 'tools',
    matches: (p) => p === '/tool/rename',
  },
  { href: '#/doctor', label: 'Doctor', glyph: '◊', section: 'system', matches: (p) => p === '/doctor' },
  { href: '#/jobs', label: 'Jobs', glyph: '⌗', section: 'system', matches: (p) => p === '/jobs' },
  {
    href: '#/settings',
    label: 'Settings',
    glyph: '⚙',
    section: 'system',
    matches: (p) => p === '/settings',
  },
];

/** pageTitle resolves the topbar breadcrumb title for the current path. */
export function pageTitle(path: string): string {
  if (path === '/' || path === '') return 'Home';
  if (path === '/doctor') return 'Doctor';
  if (path === '/jobs') return 'Jobs';
  if (path === '/settings') return 'Settings';
  if (path.startsWith('/tool/')) {
    const item = navItems.find((n) => n.matches(path));
    return item ? item.label : 'Tool';
  }
  return 'Handy Tools';
}
