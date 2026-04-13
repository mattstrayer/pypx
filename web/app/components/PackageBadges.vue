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

    <!-- Type support badge -->
    <span
      v-if="typeStatus && typeStatus !== 'untyped'"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1"
      :class="
        typeStatus === 'typed'
          ? 'bg-blue-950 text-blue-300 ring-blue-800'
          : 'bg-neutral-800 text-neutral-300 ring-neutral-700'
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
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-green-950 text-green-300 ring-green-800 hover:bg-green-900 transition-colors"
    >
      conda
    </a>

    <!-- Security badge (only shown when vulnerabilities exist) -->
    <a
      v-if="security && vulnCount > 0"
      :href="`https://osv.dev/list?ecosystem=PyPI&q=${pkg.name}`"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-red-950 text-red-300 ring-red-800 hover:bg-red-900 transition-colors"
    >
      {{ vulnCount }} {{ vulnCount === 1 ? "CVE" : "CVEs" }}
    </a>

    <!-- Maintenance status badge -->
    <span
      v-if="maintenanceStatus === 'possibly_unmaintained'"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-amber-950 text-amber-300 ring-amber-800"
    >
      Possibly Unmaintained
    </span>
    <span
      v-if="maintenanceStatus === 'likely_unmaintained'"
      class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium ring-1 bg-red-950 text-red-300 ring-red-800"
    >
      Likely Unmaintained
    </span>
  </div>
</template>
