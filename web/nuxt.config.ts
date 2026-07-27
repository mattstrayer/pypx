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
    '@nuxt/fonts',
  ],

  // Geist/Geist Mono used to be pulled in with an `@import url(...)` to Google
  // Fonts at the top of main.css. That import never survived the Vite build —
  // the deployed CSS contained zero @font-face rules, so every page silently
  // rendered in the system fallback. @nuxt/fonts downloads the files at build
  // time and serves them from this origin instead, which also keeps the CSP
  // at `font-src 'self'` and removes a render-blocking third-party request.
  fonts: {
    defaults: { subsets: ['latin'] },
    families: [
      { name: 'Geist', provider: 'google', weights: [400, 500, 600, 700] },
      { name: 'Geist Mono', provider: 'google', weights: [400, 500, 600, 700] },
    ],
  },

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

  // Note for maintainers: @nuxtjs/robots strips comments out of
  // web/public/_robots.txt when generating the served /robots.txt, so any
  // pointer note added there (e.g. "see llms.txt") is invisible to humans
  // reading the actual output. Put maintainer-facing guidance about the
  // served file's content in groups[].comment below instead — those DO
  // survive into the rendered robots.txt.
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
      ads: {
        publisher: process.env.NUXT_PUBLIC_ADS_PUBLISHER || '',
        type: process.env.NUXT_PUBLIC_ADS_TYPE || 'image',
      },
    },
  },

})
