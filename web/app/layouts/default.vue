<script setup lang="ts">
const route = useRoute();
const isHomepage = computed(() => route.path === "/");
</script>

<template>
  <div class="flex min-h-screen flex-col bg-base text-primary">
    <AppHeader :hide-search="isHomepage" />
    <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8">
      <slot />
    </main>
    <footer aria-label="Site footer" class="mt-auto border-t border-subtle py-4">
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 text-xs text-muted">
        <span>pypx — not affiliated with PyPI or the PSF</span>
        <span class="hidden sm:block">data from pypi.org · pypistats.org · osv.dev</span>
      </div>
    </footer>
  </div>
</template>
