<script setup lang="ts">
const route = useRoute();
const isHomepage = computed(() => route.path === "/");

const sources = [
  { label: "PyPI", href: "https://pypi.org" },
  { label: "pypistats.org", href: "https://pypistats.org" },
  { label: "OSV.dev", href: "https://osv.dev" },
  { label: "GitHub", href: "https://github.com" },
  { label: "GitLab", href: "https://gitlab.com" },
  { label: "conda-forge", href: "https://conda-forge.org" },
  {
    label: "top-pypi-packages",
    href: "https://github.com/hugovk/top-pypi-packages",
  },
];
</script>

<template>
  <div class="flex min-h-screen flex-col bg-base text-primary">
    <AppHeader :hide-search="isHomepage" />
    <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8">
      <slot />
    </main>
    <footer aria-label="Site footer" class="mt-auto border-t border-subtle py-4">
      <div
        class="mx-auto flex max-w-6xl flex-col gap-2 px-4 text-xs text-muted sm:flex-row sm:items-center sm:justify-between"
      >
        <span>
          pypx — not affiliated with PyPI or the PSF ·
          <a
            href="https://github.com/mattstrayer/pypx"
            target="_blank"
            rel="noopener noreferrer"
            class="underline decoration-dotted underline-offset-2 hover:text-primary"
            >Source on GitHub</a
          >
          ·
          <a
            href="/llms.txt"
            class="underline decoration-dotted underline-offset-2 hover:text-primary"
            >API for agents</a
          >
        </span>
        <!-- gap-x-2 + py-1 on the links gives each one a >=24px tap target with
             enough separation for the target-size audit on small screens. -->
        <span class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <!-- No opacity utility here: text-muted sits close to the AA floor,
               so dimming it drops this below 4.5:1. -->
          <span>data from</span>
          <template v-for="(s, i) in sources" :key="s.href">
            <a
              :href="s.href"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-block py-1 underline decoration-dotted underline-offset-2 hover:text-primary"
              >{{ s.label }}</a
            >
            <!-- Decorative separator — hidden from assistive tech, and exempt
                 from the contrast rule because it carries no information. -->
            <span v-if="i < sources.length - 1" aria-hidden="true" class="opacity-50">·</span>
          </template>
        </span>
        <!-- Nick Launches requires this backlink to stay live for the listing to
             remain published on the free tier, so treat it as load-bearing.
             `rel` is deliberately noopener WITHOUT noreferrer, unlike the links
             above: stripping the Referer header would hide pypx from their
             referral attribution, which is the traffic we get back.
             Both badge variants ship with an opaque background, so neither one
             reads correctly on the opposite theme — hence the dark: swap rather
             than a single image. -->
        <a
          href="https://nicklaunches.com/"
          target="_blank"
          rel="noopener"
          aria-label="Featured on Nick Launches"
          class="inline-block shrink-0 py-1"
        >
          <img
            src="https://nicklaunches.com/badges/featured.png"
            alt=""
            width="240"
            height="56"
            loading="lazy"
            decoding="async"
            class="h-14 w-auto dark:hidden"
          />
          <img
            src="https://nicklaunches.com/badges/featured-dark.png"
            alt=""
            width="240"
            height="56"
            loading="lazy"
            decoding="async"
            class="hidden h-14 w-auto dark:block"
          />
        </a>
      </div>
    </footer>
  </div>
</template>
