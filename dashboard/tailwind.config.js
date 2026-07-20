/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#0a0a0a',
        paper: '#fdf6e3',
        brand: '#ffde59', // yellow
        blaze: '#ff6b6b', // red/coral
        sky: '#4d9de0', // blue
        mint: '#3ddc97', // green
        grape: '#b388eb', // purple
        peach: '#ffa552',
      },
      boxShadow: {
        brutal: '4px 4px 0 0 #0a0a0a',
        'brutal-sm': '2px 2px 0 0 #0a0a0a',
        'brutal-lg': '6px 6px 0 0 #0a0a0a',
        'brutal-xl': '8px 8px 0 0 #0a0a0a',
      },
      fontFamily: {
        sans: ['"Space Grotesk"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
    },
  },
  plugins: [],
};
