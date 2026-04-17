<script setup lang="ts">
const route = useRoute();
const isHomepage = computed(() => route.path === "/");
</script>

<template>
  <div class="min-h-screen bg-base text-primary">
    <AppHeader :hide-search="isHomepage" />
    <main class="mx-auto max-w-6xl px-4 py-8">
      <slot />
    </main>
  </div>
</template>
