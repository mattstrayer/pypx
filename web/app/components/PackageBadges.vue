<script setup lang="ts">
import { computed } from "vue";
import type { PackageData, ExtrasData, SecurityData } from "~/types/api";
import type { MaintenanceStatus } from "~/composables/useMaintenanceStatus";

const props = defineProps<{
  pkg: PackageData;
  extras?: ExtrasData | null;
  security?: SecurityData | null;
  maintenanceStatus?: MaintenanceStatus;
}>();

function formatSize(bytes: number): string {
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
}

const vulnCount = computed(() => props.security?.vulns?.length ?? 0);
const typeStatus = computed(() => props.extras?.type_support?.status);
const condaAvailable = computed(() => props.extras?.conda_forge?.available);
const condaUrl = computed(() => props.extras?.conda_forge?.url ?? null);
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <span
      v-if="pkg.install_size"
      class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
    >
      {{ formatSize(pkg.install_size) }}
    </span>
    <span
      v-if="pkg.python_versions?.min_version"
      class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
    >
      Python {{ pkg.python_versions.min_version }}+
    </span>
    <span
      v-if="pkg.module_format && pkg.module_format !== 'sdist-only'"
      class="inline-flex items-center rounded bg-[var(--color-brand-muted)] px-2 py-0.5 font-mono text-xs text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]"
    >
      {{ pkg.module_format }}
    </span>
    <span
      v-if="pkg.license"
      class="inline-flex items-center rounded bg-raised/50 px-2 py-0.5 font-mono text-xs text-muted ring-1 ring-subtle/50"
    >
      {{ pkg.license }}
    </span>
    <span
      v-if="pkg.dependencies?.required?.length"
      class="inline-flex items-center rounded bg-amber-50 px-2 py-0.5 font-mono text-xs text-amber-700 ring-1 ring-amber-200 dark:bg-amber-950 dark:text-amber-400 dark:ring-amber-800"
    >
      {{ pkg.dependencies.required.length }} deps
    </span>

    <!-- Type support badge -->
    <span
      v-if="typeStatus && typeStatus !== 'untyped'"
      class="inline-flex items-center gap-1 rounded px-2 py-0.5 font-mono text-xs ring-1"
      :class="
        typeStatus === 'typed'
          ? 'bg-blue-50 text-blue-700 ring-blue-200 dark:bg-blue-950 dark:text-blue-300 dark:ring-blue-800'
          : 'bg-zinc-100 text-zinc-600 ring-zinc-200 dark:bg-neutral-800 dark:text-neutral-300 dark:ring-neutral-700'
      "
    >
      <span v-if="typeStatus === 'typed'">typed</span>
      <span v-else>type stubs</span>
    </span>

    <!-- Conda badge -->
    <a
      v-if="condaAvailable && condaUrl"
      :href="condaUrl"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 rounded px-2 py-0.5 font-mono text-xs ring-1 transition-colors bg-emerald-50 text-emerald-700 ring-emerald-200 hover:bg-emerald-100 dark:bg-green-950 dark:text-green-300 dark:ring-green-800 dark:hover:bg-green-900"
    >
      conda
    </a>

    <!-- Security badge (only shown when vulnerabilities exist) -->
    <a
      v-if="security && vulnCount > 0"
      :href="`https://osv.dev/list?ecosystem=PyPI&q=${pkg.name}`"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 rounded px-2 py-0.5 font-mono text-xs ring-1 transition-colors bg-red-50 text-red-700 ring-red-200 hover:bg-red-100 dark:bg-red-950 dark:text-red-300 dark:ring-red-800 dark:hover:bg-red-900"
    >
      {{ vulnCount }} {{ vulnCount === 1 ? "CVE" : "CVEs" }}
    </a>

    <!-- Maintenance status badge -->
    <span
      v-if="maintenanceStatus === 'possibly_unmaintained'"
      class="inline-flex items-center gap-1 rounded px-2 py-0.5 font-mono text-xs ring-1 bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:ring-amber-800"
    >
      Possibly Unmaintained
    </span>
    <span
      v-if="maintenanceStatus === 'likely_unmaintained'"
      class="inline-flex items-center gap-1 rounded px-2 py-0.5 font-mono text-xs ring-1 bg-red-50 text-red-700 ring-red-200 dark:bg-red-950 dark:text-red-300 dark:ring-red-800"
    >
      Likely Unmaintained
    </span>
  </div>
</template>
