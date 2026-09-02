/** @type {import('tailwindcss').Config} */
// The Modernist system (D39). Every colour here is a measured value from
// design.md §2-§3; nothing is chosen at a call site.
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg:      '#f3f2f2',
        surface: '#eae9e9',
        paper:   '#ffffff',
        ink:     '#201e1d',
        // rgba(32,30,29,.65) flattened on the page ground. 4.96 on bg.
        muted:   'rgba(32,30,29,0.65)',
        // #ec3013 is a FILL, never text: 3.76 passes as a rule and fails as
        // an ink. accent-ink is what carries words.
        accent:      '#ec3013',
        'accent-ink':   '#ae1800',
        'accent-hover': '#7c1405',
        'accent-100': '#fff2ef',
        'accent-200': '#ffe0d9',
        divider: 'rgba(32,30,29,0.55)',
        hairline: 'rgba(32,30,29,0.18)',
        success: '#145f38',
        warn:    '#6f4400',
        danger:  '#9e1c28',
        info:    '#0f5f73',
        'brand-maxx':     '#6b3b2a',
        'brand-ruuma':    '#7a2e63',
        'brand-sunshine': '#7c4a00',
        'neutral-100': '#f8f4f4',
        'neutral-200': '#eae7e7',
        'neutral-300': '#d7d3d3',
        'neutral-800': '#444141',
        'neutral-900': '#2d2b2b',
      },
      fontFamily: {
        sans: ['Archivo', 'system-ui', 'sans-serif'],
      },
      borderRadius: {
        // Radius is 0 everywhere. It is the system's signature.
        none: '0', sm: '0', DEFAULT: '0', md: '0', lg: '0', xl: '0', full: '0',
      },
      fontSize: {
        xs: ['11px', '1.4'], sm: ['13px', '1.5'], base: ['15px', '1.55'],
        lg: ['20px', '1.2'], xl: ['25px', '1.15'], '2xl': ['32px', '1.12'],
        '3xl': ['42px', '1.1'],
      },
    },
  },
  plugins: [],
}
