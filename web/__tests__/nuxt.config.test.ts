/**
 * Tests for nuxt.config.ts static values.
 *
 * We stub `defineNuxtConfig` as an identity function so the config module can
 * be imported without a full Nuxt build context, letting us assert against the
 * raw config object.
 */

// @vitest-environment node

import { describe, it, expect, vi, beforeAll } from 'vitest'

// Stub defineNuxtConfig before importing the config file.
vi.stubGlobal('defineNuxtConfig', (config: Record<string, unknown>) => config)

// Dynamic import must come after the stub.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let config: any

beforeAll(async () => {
  const mod = await import('../nuxt.config')
  config = mod.default
})

describe('nuxt.config.ts — devtools', () => {
  it('disables devtools in production builds', () => {
    expect(config.devtools?.enabled).toBe(false)
  })
})

describe('nuxt.config.ts — routeRules cache headers', () => {
  it('/packages/** has correct Cache-Control for cacheable package pages', () => {
    const rule = config.routeRules?.['/packages/**']
    expect(rule).toBeDefined()
    expect(rule.headers?.['Cache-Control']).toBe(
      'public, max-age=60, stale-while-revalidate=300'
    )
  })

  it('/ (home) has short-lived public cache with stale-while-revalidate', () => {
    const rule = config.routeRules?.['/']
    expect(rule).toBeDefined()
    expect(rule.headers?.['Cache-Control']).toBe(
      'public, max-age=30, stale-while-revalidate=120'
    )
  })

  it('/search is not cached (no-store) to prevent stale search results', () => {
    const rule = config.routeRules?.['/search']
    expect(rule).toBeDefined()
    expect(rule.headers?.['Cache-Control']).toBe('no-store')
  })

  it('package pages have longer max-age than home page', () => {
    const pkgCC = config.routeRules?.['/packages/**']?.headers?.['Cache-Control'] ?? ''
    const homeCC = config.routeRules?.['/']?.headers?.['Cache-Control'] ?? ''

    const pkgMaxAge = Number(pkgCC.match(/max-age=(\d+)/)?.[1] ?? 0)
    const homeMaxAge = Number(homeCC.match(/max-age=(\d+)/)?.[1] ?? 0)

    expect(pkgMaxAge).toBeGreaterThan(homeMaxAge)
  })

  it('all three route rules are defined (no missing entries)', () => {
    const rules = config.routeRules ?? {}
    expect(Object.keys(rules)).toContain('/packages/**')
    expect(Object.keys(rules)).toContain('/')
    expect(Object.keys(rules)).toContain('/search')
  })
})

describe('nuxt.config.ts — runtimeConfig', () => {
  it('has a server-side apiBase with a sensible default', () => {
    const base = config.runtimeConfig?.apiBase
    expect(typeof base).toBe('string')
    expect(base.length).toBeGreaterThan(0)
  })

  it('has a public apiBase for client-side use', () => {
    const base = config.runtimeConfig?.public?.apiBase
    expect(typeof base).toBe('string')
    expect(base.length).toBeGreaterThan(0)
  })
})
