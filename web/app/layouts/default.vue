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
        <span class="flex flex-wrap gap-x-1 gap-y-0.5">
          <span class="opacity-70">data from</span>
          <template v-for="(s, i) in sources" :key="s.href">
            <a
              :href="s.href"
              target="_blank"
              rel="noopener noreferrer"
              class="underline decoration-dotted underline-offset-2 hover:text-primary"
              >{{ s.label }}</a
            >
            <span v-if="i < sources.length - 1" class="opacity-50">·</span>
          </template>
        </span>
      </div>
    </footer>
  </div>
</template>
