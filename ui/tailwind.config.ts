import type { Config } from 'tailwindcss';

const config: Config = {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        'a-bg': '#181225',
        'a-surface': '#1e1633',
        'a-surface-warm': '#251d3d',
        'a-fg': '#f0ecf8',
        'a-fg2': '#c9c0dc',
        'a-muted': '#8b83a6',
        'a-meta': '#b894f5',
        'a-border': '#2d2447',
        'a-border-soft': '#231c3a',
        'a-accent': '#a865ff',
        'a-accent-hover': '#c08fff',
        'a-accent-active': '#8b3fe0',
        'a-success': '#4cd964',
        'a-warn': '#e8b830',
        'a-danger': '#ff5c72',
      },
      fontFamily: {
        body: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SF Mono', 'Menlo', 'monospace'],
      },
      borderRadius: {
        'a': '10px',
        'a-sm': '8px',
        'a-md': '12px',
        'a-lg': '16px',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
};

export default config;
