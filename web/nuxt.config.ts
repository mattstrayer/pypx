export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: true },

  modules: [
    '@nuxt/ui',
    '@vueuse/nuxt',
    '@nuxtjs/mdc',
  ],

  css: ['~/assets/css/main.css'],

  colorMode: {
    preference: 'dark',
    fallback: 'dark',
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
