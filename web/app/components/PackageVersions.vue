<script setup lang="ts">
import type { ChangelogEntry } from "~/types/api";

const props = defineProps<{
  name: string;
}>();

const { fetchVersions, fetchChangelog } = useApi();
const { data: versions, status } = await useAsyncData(`versions-${props.name}`, () =>
  fetchVersions(props.name),
);

const { data: changelog } = await useAsyncData(`changelog-${props.name}`, () =>
  fetchChangelog(props.name),
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

const expandedVersions = ref(new Set<string>());

function toggleVersion(version: string) {
  const next = new Set(expandedVersions.value);
  if (next.has(version)) next.delete(version);
  else next.add(version);
  expandedVersions.value = next;
}

function formatSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${bytes} B`;
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
      <div class="h-6 w-6 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <!-- Table -->
    <table v-else class="w-full text-sm">
      <thead>
        <tr class="border-b border-zinc-800 text-left text-zinc-500">
          <th class="pb-2 pr-6 font-medium">Version</th>
          <th class="pb-2 pr-6 font-medium">Released</th>
          <th class="pb-2 pr-6 font-medium">Size</th>
          <th class="pb-2 font-medium">Format</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="v in sortedVersions" :key="v.version">
          <tr
            class="border-b border-zinc-800 last:border-0"
            :class="{ 'cursor-pointer hover:bg-zinc-800/30': changelogMap.has(v.version) }"
            @click="changelogMap.has(v.version) ? toggleVersion(v.version) : undefined"
          >
            <td class="py-3 pr-6">
              <div class="flex items-center gap-2">
                <span
                  v-if="changelogMap.has(v.version)"
                  class="text-xs text-zinc-500 select-none"
                  >{{ expandedVersions.has(v.version) ? "▼" : "▶" }}</span
                >
                <NuxtLink
                  :to="`/packages/${name}/${v.version}`"
                  class="font-mono hover:text-indigo-400 text-zinc-200 transition-colors"
                  @click.stop
                >
                  {{ v.version }}
                </NuxtLink>
              </div>
            </td>
            <td class="py-3 pr-6 text-zinc-400">{{ formatDate(v.upload_time) }}</td>
            <td class="py-3 pr-6 font-mono text-emerald-400">{{ formatSize(v.install_size) }}</td>
            <td class="py-3 font-mono text-xs text-zinc-500">{{ v.module_format || "—" }}</td>
          </tr>
          <tr v-if="expandedVersions.has(v.version) && changelogMap.has(v.version)">
            <td colspan="4" class="pb-4 pt-1">
              <div
                v-for="entry in [changelogMap.get(v.version)!]"
                :key="entry.version"
                class="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4"
              >
                <h4 class="text-sm font-semibold text-zinc-200 mb-1">{{ entry.title }}</h4>
                <p class="text-xs text-zinc-500 mb-3">{{ formatDate(entry.published_at) }}</p>
                <div class="prose prose-invert prose-sm max-w-none mb-3">
                  <div v-if="entry.body_html" v-html="entry.body_html" />
                  <div v-else class="whitespace-pre-wrap">{{ entry.body }}</div>
                </div>
                <a
                  :href="entry.url"
                  target="_blank"
                  class="text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
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
