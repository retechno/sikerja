/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      fontFamily: { sans: ['"Plus Jakarta Sans"', 'system-ui', 'sans-serif'] },
      colors: {
        primary: { DEFAULT: '#0E5A52', dark: '#09403A', soft: '#E3EFEC', tint: '#F3F8F6' },
        accent: '#E8A33D',
        side: { DEFAULT: '#0F2421', hover: '#18332F', text: '#C9D6D2', label: '#8FA6A1' },
        canvas: '#F4F5F1',
        ink: { DEFAULT: '#18201E', soft: '#5B6662' },
        line: { DEFAULT: '#E3E6E0', soft: '#EEF0EC', input: '#CBD1CA' },
        good: { bg: '#E2F2E8', fg: '#14583A' },
        info: { bg: '#E3EDFA', fg: '#174E8C' },
        warn: { bg: '#FDF0D8', fg: '#7A4700' },
        bad: { bg: '#FDE7E4', fg: '#9B1C12' },
        mute: { bg: '#EEF0EC', fg: '#454F4B' },
      },
      borderRadius: { card: '16px' },
    },
  },
  plugins: [],
}
