<script setup lang="ts">
import type { ChangelogEntry } from "~/types/api";

const props = defineProps<{
  name: string;
}>();

const { fetchVersions, fetchChangelog } = useApi();
const { data: versions, status } = useAsyncData(
  () => `versions-${props.name}`,
  () => fetchVersions(props.name),
  { watch: [() => props.name] },
);

const { data: changelog } = useAsyncData(
  () => `changelog-${props.name}`,
  () => fetchChangelog(props.name),
  { server: false, default: () => null, watch: [() => props.name] },
);

const sortedVersions = computed(() => {
  if (!versions.value) return [];
  return [...versions.value].sort(
    (a, b) => new Date(b.upload_time).getTime() - new Date(a.upload_time).getTime(),
  );
});

const changelogMap = computed(() => {
  const map = new Map<string, ChangelogEntry>();
  if (changelog.value?.entries) {
    for (const entry of changelog.value.entries) {
      map.set(entry.version, entry);
    }
  }
  return map;
});

const sourceBadgeLabel = computed(() => {
  const labels: Record<string, string> = {
    github_changelog_file: "via CHANGELOG",
    github_tags: "via git tags",
    gitlab_releases: "via GitLab",
    gitlab_changelog_file: "via CHANGELOG",
    gitlab_tags: "via git tags",
  };
  return labels[changelog.value?.source ?? ""] ?? "";
});

const showSourceBadge = computed(
  () => sourceBadgeLabel.value !== "" && (changelog.value?.entries?.length ?? 0) > 0,
);

const expandedVersions = ref(new Set<string>());

function toggleVersion(version: string) {
  const next = new Set(expandedVersions.value);
  if (next.has(version)) next.delete(version);
  else next.add(version);
  expandedVersions.value = next;
}

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-16">
      <div class="h-6 w-6 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Source badge: shown when changelog data comes from a non-default source -->
    <div v-if="showSourceBadge" class="flex items-center justify-end mb-2">
      <span
        class="flex items-center gap-1 bg-raised border border-subtle text-muted text-[10px] px-1.5 py-0.5 rounded font-mono"
      >
        <!-- Tag icon for tag-based sources -->
        <svg
          v-if="changelog?.source?.includes('tags')"
          xmlns="http://www.w3.org/2000/svg"
          class="w-2.5 h-2.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path
            d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"
          />
          <line x1="7" y1="7" x2="7.01" y2="7" />
        </svg>
        <!-- Document icon for file-based sources -->
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="w-2.5 h-2.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
          <polyline points="14 2 14 8 20 8" />
        </svg>
        {{ sourceBadgeLabel }}
      </span>
    </div>

    <!-- Table -->
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-subtle text-left text-muted">
          <th class="pb-2 pr-6 font-medium">Version</th>
          <th class="pb-2 pr-6 font-medium">Released</th>
          <th class="hidden pb-2 pr-6 font-medium sm:table-cell">Size</th>
          <th class="hidden pb-2 font-medium sm:table-cell">Format</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="v in sortedVersions" :key="v.version">
          <tr
            class="border-b border-subtle last:border-0"
            :class="{ 'cursor-pointer hover:bg-raised/30': changelogMap.has(v.version) }"
            @click="changelogMap.has(v.version) ? toggleVersion(v.version) : undefined"
          >
            <td class="py-3 pr-6">
              <div class="flex items-center gap-2">
                <span v-if="changelogMap.has(v.version)" class="text-xs text-muted select-none">{{
                  expandedVersions.has(v.version) ? "▼" : "▶"
                }}</span>
                <NuxtLink
                  :to="`/packages/${name}/${v.version}`"
                  class="font-mono hover:text-[var(--color-brand)] text-zinc-800 dark:text-zinc-200 transition-colors"
                  @click.stop
                >
                  {{ v.version }}
                </NuxtLink>
              </div>
            </td>
            <td class="py-3 pr-6 text-muted">{{ formatDate(v.upload_time) }}</td>
            <td class="hidden py-3 pr-6 font-mono text-[var(--color-brand)] sm:table-cell">
              {{ formatSize(v.install_size) }}
            </td>
            <td class="hidden py-3 font-mono text-xs text-muted sm:table-cell">
              {{ v.module_format || "—" }}
            </td>
          </tr>
          <tr v-if="expandedVersions.has(v.version) && changelogMap.has(v.version)">
            <td colspan="4" class="pb-4 pt-1">
              <div
                v-for="entry in [changelogMap.get(v.version)!]"
                :key="entry.version"
                class="bg-surface border border-subtle rounded-lg p-4"
              >
                <h4 class="text-sm font-semibold text-zinc-800 dark:text-zinc-200 mb-1">
                  {{ entry.title }}
                </h4>
                <p class="text-xs text-muted mb-3">{{ formatDate(entry.published_at) }}</p>
                <div class="prose prose-sm max-w-none mb-3">
                  <div v-if="entry.body_html" v-html="entry.body_html" />
                  <div v-else class="whitespace-pre-wrap">{{ entry.body }}</div>
                </div>
                <a
                  :href="entry.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-xs text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
                  >View on GitHub →</a
                >
              </div>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>
