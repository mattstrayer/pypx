export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: true },

  modules: [
    '@vueuse/nuxt',
    '@nuxtjs/seo',
    '@nuxtjs/color-mode',
  ],

  colorMode: {
    classSuffix: '',
    defaultValue: 'system',
    storageKey: 'pypx-color-mode',
  },

  css: ['~/assets/css/main.css'],

  app: {
    head: {
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
      ],
    },
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
    enabled: true,
  },

  schemaOrg: {
    reactive: true,
  },

  sitemap: {
    sitemaps: {
      popular: {
        sources: ["/api/__sitemap__/urls"],
      },
      cached: {
        sources: ["/api/__sitemap__/urls"],
      },
    },
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

})
