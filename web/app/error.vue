<script setup lang="ts">
import type { NuxtError } from "#app";

const props = defineProps<{ error: NuxtError }>();

const status = computed(() => props.error.status ?? props.error.statusCode ?? 500);
const heading = computed(() => (status.value === 404 ? "Page not found" : "Something went wrong"));

useSeoMeta({ title: heading });

function goHome() {
  clearError({ redirect: "/" });
}
</script>

<template>
  <NuxtLayout>
    <div class="py-24 text-center">
      <p class="font-mono text-5xl font-bold text-primary">{{ status }}</p>
      <h1 class="mt-4 text-lg font-medium text-zinc-700 dark:text-zinc-300">{{ heading }}</h1>
      <p class="mt-1 text-sm text-muted">
        {{
          status === 404
            ? "Try searching for a package using the search bar above."
            : "Please try again soon."
        }}
      </p>
      <button
        class="mt-6 cursor-pointer rounded-md border border-subtle bg-raised px-4 py-2 text-sm font-medium text-primary transition-colors hover:border-[var(--color-brand)]"
        @click="goHome"
      >
        Back to home
      </button>
    </div>
  </NuxtLayout>
  <CommandPalette />
</template>
