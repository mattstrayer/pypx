<script setup lang="ts">
import type { PackageData } from "~/types/api";

defineProps<{ pkg: PackageData }>();

function formatSize(bytes: number): string {
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <span
      v-if="pkg.install_size"
      class="inline-flex items-center rounded bg-emerald-500/10 px-2 py-0.5 font-mono text-xs text-emerald-400 ring-1 ring-emerald-500/20"
    >
      {{ formatSize(pkg.install_size) }}
    </span>
    <span
      v-if="pkg.python_versions?.min_version"
      class="inline-flex items-center rounded bg-indigo-500/10 px-2 py-0.5 font-mono text-xs text-indigo-400 ring-1 ring-indigo-500/20"
    >
      Python {{ pkg.python_versions.min_version }}+
    </span>
    <span
      v-if="pkg.module_format && pkg.module_format !== 'sdist-only'"
      class="inline-flex items-center rounded bg-indigo-500/10 px-2 py-0.5 font-mono text-xs text-indigo-400 ring-1 ring-indigo-500/20"
    >
      {{ pkg.module_format }}
    </span>
    <span
      v-if="pkg.license"
      class="inline-flex items-center rounded bg-zinc-500/10 px-2 py-0.5 font-mono text-xs text-zinc-400 ring-1 ring-zinc-500/20"
    >
      {{ pkg.license }}
    </span>
    <span
      v-if="pkg.dependencies?.required?.length"
      class="inline-flex items-center rounded bg-amber-500/10 px-2 py-0.5 font-mono text-xs text-amber-400 ring-1 ring-amber-500/20"
    >
      {{ pkg.dependencies.required.length }} deps
    </span>
  </div>
</template>
