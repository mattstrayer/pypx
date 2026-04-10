export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: true },

  modules: [
    '@nuxt/ui',
    '@vueuse/nuxt',
    '@nuxtjs/mdc',
    '@nuxtjs/seo',
  ],

  css: ['#build/ui.css', '~/assets/css/main.css'],

  colorMode: {
    preference: 'dark',
    fallback: 'dark',
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
    '/api/_mdc/**': {},
    '/api/_nuxt_icon/**': {},
    '/api/**': {
      proxy: { to: `${process.env.API_BASE || 'http://localhost:8080'}/**` },
    },
  },
})
