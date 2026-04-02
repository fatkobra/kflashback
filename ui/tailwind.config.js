/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    fontSize: {
      xs: ['0.6875rem', { lineHeight: '1rem' }],      // 11px
      sm: ['0.75rem', { lineHeight: '1.125rem' }],     // 12px
      base: ['0.8125rem', { lineHeight: '1.25rem' }],  // 13px
      lg: ['0.875rem', { lineHeight: '1.375rem' }],    // 14px
      xl: ['1rem', { lineHeight: '1.5rem' }],           // 16px
      '2xl': ['1.125rem', { lineHeight: '1.625rem' }], // 18px
    },
    extend: {
      colors: {
        brand: {
          50: '#eff6ff',
          100: '#dbeafe',
          200: '#bfdbfe',
          300: '#93c5fd',
          400: '#60a5fa',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
          800: '#1e40af',
          900: '#1e3a8a',
          950: '#172554',
        },
      },
    },
  },
  plugins: [],
}
