export default defineNuxtConfig({
  compatibilityDate: '2026-04-09',
  devtools: { enabled: false },

  routeRules: {
    '/packages/**': {
      headers: { 'Cache-Control': 'public, max-age=60, stale-while-revalidate=300' },
    },
    '/': {
      headers: { 'Cache-Control': 'public, max-age=30, stale-while-revalidate=120' },
    },
  },

  modules: [
    '@vueuse/nuxt',
    '@nuxtjs/seo',
    '@nuxtjs/color-mode',
  ],

  colorMode: {
    classSuffix: '',
    preference: 'system',
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
    groups: [
      {
        userAgent: ['*'],
        comment: [
          'pypx is agent-first. Machine-readable index: https://pypx.app/llms.txt',
          'Plain-text API: https://pypx.app/api/packages/{name}.txt (see llms.txt)',
        ],
      },
    ],
  },

  runtimeConfig: {
    apiBase: process.env.NUXT_API_BASE || 'http://localhost:8080/api',
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '/api',
    },
  },

})
