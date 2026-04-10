export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: true },

  modules: [
    '@nuxtjs/color-mode',
    '@vueuse/nuxt',
    '@nuxtjs/seo',
  ],

  css: ['~/assets/css/main.css'],

  colorMode: {
    classSuffix: '',
    preference: 'dark',
    fallback: 'dark',
  },

  vite: {
    plugins: [
      import('@tailwindcss/vite').then((m) => m.default()),
    ],
  },

  site: {
    url: 'https://pypx.app',
    name: 'pypx',
    description: 'The Python Package Index, reimagined. Fast search, dependency insights, download trends, and changelogs — all in one place.',
    defaultLocale: 'en',
  },

  ogImage: {
    enabled: false,
  },

  robots: {
    allow: '/',
  },

  runtimeConfig: {
    apiBase: process.env.API_BASE || 'http://localhost:8080',
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api',
    },
  },

  routeRules: {
    '/api/**': {
      proxy: { to: `${process.env.API_BASE || 'http://localhost:8080'}/**` },
    },
  },
})
