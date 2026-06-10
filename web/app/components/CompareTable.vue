<script setup lang="ts">
import type { ComparePackageData } from "~/types/api";

const props = defineProps<{
  packages: ComparePackageData[];
}>();

function formatInstallSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDownloads(n: number): string {
  if (!n) return "—";
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}K`;
  if (n < 1_000_000_000) return `${Math.floor(n / 1_000_000)}M`;
  return `${(n / 1_000_000_000).toFixed(1)}B`;
}

interface Row {
  label: string;
  values: string[];
}

const rows = computed<Row[]>(() => [
  { label: "Version", values: props.packages.map((p) => p.Version || "—") },
  { label: "Summary", values: props.packages.map((p) => p.Summary || "—") },
  { label: "License", values: props.packages.map((p) => p.License || "—") },
  { label: "Python min", values: props.packages.map((p) => p.PythonMin || "—") },
  {
    label: "Install size",
    values: props.packages.map((p) => formatInstallSize(p.InstallSize)),
  },
  { label: "Module format", values: props.packages.map((p) => p.ModuleFormat || "—") },
  {
    label: "Last release",
    values: props.packages.map((p) => p.LastReleasedDate || "—"),
  },
  {
    label: "Releases/12mo",
    values: props.packages.map((p) => (p.ReleasesLast12Mo ? String(p.ReleasesLast12Mo) : "—")),
  },
  {
    label: "Dependencies",
    values: props.packages.map((p) => (p.DepCount ? String(p.DepCount) : "—")),
  },
  {
    label: "Downloads/30d",
    values: props.packages.map((p) => formatDownloads(p.Downloads30d)),
  },
  {
    label: "Vulns",
    values: props.packages.map((p) => String(p.VulnCount ?? 0)),
  },
  { label: "Typed", values: props.packages.map((p) => p.Typed || "—") },
]);
</script>

<template>
  <div class="overflow-x-auto rounded-[14px] border border-subtle bg-surface font-mono text-sm">
    <table class="w-full min-w-[480px] border-collapse">
      <thead>
        <tr class="border-b border-subtle bg-raised/40">
          <th class="py-2.5 pl-4 pr-3 text-left text-[10px] uppercase tracking-[0.1em] text-muted">
            metric
          </th>
          <th
            v-for="pkg in packages"
            :key="pkg.Name"
            class="py-2.5 px-4 text-left text-xs font-semibold text-primary"
          >
            <NuxtLink
              :to="`/packages/${pkg.Name}`"
              class="text-[var(--color-brand)] hover:underline"
            >
              {{ pkg.Name }}
            </NuxtLink>
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-subtle">
        <tr
          v-for="row in rows"
          :key="row.label"
          class="transition-colors hover:bg-[var(--color-brand-muted)]"
        >
          <td class="py-2 pl-4 pr-3 text-[11px] uppercase tracking-[0.07em] text-muted">
            {{ row.label }}
          </td>
          <td
            v-for="(val, i) in row.values"
            :key="i"
            class="max-w-[16rem] truncate px-4 py-2 text-xs text-primary"
            :title="val !== '—' ? val : undefined"
          >
            {{ val }}
          </td>
        </tr>
        <!-- Links row -->
        <tr class="transition-colors hover:bg-[var(--color-brand-muted)]">
          <td class="py-2 pl-4 pr-3 text-[11px] uppercase tracking-[0.07em] text-muted">Docs</td>
          <td v-for="pkg in packages" :key="`doc-${pkg.Name}`" class="px-4 py-2 text-xs">
            <a
              v-if="pkg.DocURL"
              :href="pkg.DocURL"
              target="_blank"
              rel="noopener noreferrer"
              class="text-[var(--color-brand)] hover:underline"
            >
              link
            </a>
            <span v-else class="text-muted">—</span>
          </td>
        </tr>
        <tr class="transition-colors hover:bg-[var(--color-brand-muted)]">
          <td class="py-2 pl-4 pr-3 text-[11px] uppercase tracking-[0.07em] text-muted">Repo</td>
          <td v-for="pkg in packages" :key="`repo-${pkg.Name}`" class="px-4 py-2 text-xs">
            <a
              v-if="pkg.RepoURL"
              :href="pkg.RepoURL"
              target="_blank"
              rel="noopener noreferrer"
              class="text-[var(--color-brand)] hover:underline"
            >
              link
            </a>
            <span v-else class="text-muted">—</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
