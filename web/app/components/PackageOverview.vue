<script setup lang="ts">
import type { PackageData } from "~/types/api";

const props = defineProps<{
  pkg: PackageData;
}>();

const maintainer = computed(() => props.pkg.author || props.pkg.author_email || null);

const projectLinks = computed(() => {
  const urls = props.pkg.project_urls;
  if (!urls) return [];
  return Object.entries(urls).map(([label, url]) => ({ label, url }));
});
</script>

<template>
  <div class="grid gap-6 lg:grid-cols-[1fr_300px]">
    <!-- Main column -->
    <div class="min-w-0 space-y-6">
      <!-- Install command -->
      <InstallCommand :package-name="pkg.name" />

      <!-- Description -->
      <div
        v-if="pkg.description"
        class="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/50 p-5"
      >
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-zinc-500">
          Description
        </h2>
        <div
          v-if="pkg.description_html"
          class="prose prose-invert prose-sm max-w-none"
          v-html="pkg.description_html"
        />
        <div v-else class="whitespace-pre-wrap break-words text-sm leading-relaxed text-zinc-300">
          {{ pkg.description }}
        </div>
      </div>
    </div>

    <!-- Sidebar -->
    <div class="space-y-4">
      <!-- Metadata card -->
      <div class="rounded-lg border border-zinc-800 bg-zinc-900/50 p-4">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-zinc-500">Details</h2>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-2">
            <dt class="text-zinc-500">Version</dt>
            <dd class="font-mono text-zinc-300">{{ pkg.version }}</dd>
          </div>
          <div v-if="pkg.license" class="flex justify-between gap-2">
            <dt class="text-zinc-500">License</dt>
            <dd class="text-right text-zinc-300">{{ pkg.license }}</dd>
          </div>
          <div v-if="pkg.requires_python" class="flex justify-between gap-2">
            <dt class="text-zinc-500">Python</dt>
            <dd class="font-mono text-zinc-300">{{ pkg.requires_python }}</dd>
          </div>
          <div v-if="maintainer" class="flex justify-between gap-2">
            <dt class="text-zinc-500">Maintainer</dt>
            <dd class="truncate text-right text-zinc-300">{{ maintainer }}</dd>
          </div>
        </dl>
      </div>

      <!-- Project links card -->
      <div
        v-if="projectLinks.length > 0"
        class="rounded-lg border border-zinc-800 bg-zinc-900/50 p-4"
      >
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-zinc-500">Links</h2>
        <ul class="space-y-1.5 text-sm">
          <li v-for="link in projectLinks" :key="link.label">
            <a
              :href="link.url"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-1.5 text-zinc-400 transition-colors hover:text-zinc-200"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5 shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
              {{ link.label }}
            </a>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
