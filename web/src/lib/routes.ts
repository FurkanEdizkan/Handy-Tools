import type { Component } from 'svelte';
import Home from './pages/Home.svelte';
import ToolPage from './pages/ToolPage.svelte';
import Doctor from './pages/Doctor.svelte';
import Settings from './pages/Settings.svelte';
import DevComponents from './pages/DevComponents.svelte';
import NotFound from './pages/NotFound.svelte';

export const routes: Record<string, Component> = {
  '/': Home,
  '/tool/:id': ToolPage,
  '/doctor': Doctor,
  '/settings': Settings,
  // Dev-only component gallery (issue #70) — intentionally not in `navItems`.
  '/dev/components': DevComponents,
  '*': NotFound,
};

export type NavItem = {
  href: string;
  label: string;
  glyph: string;
  matches: (path: string) => boolean;
};

export const navItems: NavItem[] = [
  { href: '#/', label: 'Home', glyph: '◇', matches: (p) => p === '/' || p === '' },
  { href: '#/tool/convert', label: 'Convert', glyph: '⇄', matches: (p) => p.startsWith('/tool/') },
  { href: '#/doctor', label: 'Doctor', glyph: '✚', matches: (p) => p === '/doctor' },
  { href: '#/settings', label: 'Settings', glyph: '⚙', matches: (p) => p === '/settings' },
];
