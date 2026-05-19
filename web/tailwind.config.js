/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{html,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        bg: 'var(--color-bg)',
        surface: 'var(--color-surface)',
        'surface-2': 'var(--color-surface-2)',
        border: 'var(--color-border)',
        text: 'var(--color-text)',
        'text-dim': 'var(--color-text-dim)',
        accent: 'var(--color-accent)',
        'accent-2': 'var(--color-accent-2)',
      },
      fontFamily: {
        sans: ['Geist', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
};
