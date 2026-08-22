/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - 鼠尾草绿
        primary: {
          50: '#eff8f4',
          100: '#dceee6',
          200: '#bddccc',
          300: '#9ac9b5',
          400: '#7cb9a8',
          500: '#639d8c',
          600: '#4d8373',
          700: '#3c685b',
          800: '#315449',
          900: '#29453d',
          950: '#1b302a'
        },
        // 辅助色 - 暖棕色
        accent: {
          50: '#fbf7f0',
          100: '#f3eadb',
          200: '#e6d2b5',
          300: '#d5b991',
          400: '#bd9568',
          500: '#9d754e',
          600: '#805d3e',
          700: '#674a33',
          800: '#543e2d',
          900: '#463427',
          950: '#30231b'
        },
        // 深色模式背景 - 暮色森林
        dark: {
          50: '#f3f4e8',
          100: '#dde2cf',
          200: '#bdc9b0',
          300: '#9cac91',
          400: '#7b8a73',
          500: '#64715d',
          600: '#505b4b',
          700: '#414a3c',
          800: '#333b30',
          900: '#273025',
          950: '#1d251c'
        }
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Helvetica Neue',
          'Arial',
          'Noto Sans SC',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(92, 125, 102, 0.14)',
        'glass-sm': '0 4px 16px rgba(92, 125, 102, 0.1)',
        glow: '0 0 20px rgba(124, 185, 168, 0.25)',
        'glow-lg': '0 0 40px rgba(124, 185, 168, 0.35)',
        card: '0 4px 16px rgba(124, 185, 168, 0.16)',
        'card-hover': '0 8px 24px rgba(124, 185, 168, 0.24)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #8fcab8 0%, #6fae9d 100%)',
        'gradient-dark': 'linear-gradient(135deg, #414a3c 0%, #273025 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 20% 15%, rgba(133, 205, 202, 0.25) 0px, transparent 45%), radial-gradient(at 80% 10%, rgba(232, 168, 124, 0.18) 0px, transparent 40%), radial-gradient(at 55% 75%, rgba(124, 185, 168, 0.2) 0px, transparent 52%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(20, 184, 166, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(20, 184, 166, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
