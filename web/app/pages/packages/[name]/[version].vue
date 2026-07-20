<script setup lang="ts">
const route = useRoute();
const name = computed(() => route.params.name as string);
const version = computed(() => route.params.version as string);

const { fetchPackage, fetchVersions, fetchChangelog } = useApi();

const [{ data: pkg }, { data: versions }] = await Promise.all([
  useAsyncData(
    () => `package-${name.value}`,
    () => fetchPackage(name.value),
    { watch: [name] },
  ),
  useAsyncData(
    () => `versions-${name.value}`,
    () => fetchVersions(name.value),
    {
      watch: [name],
    },
  ),
]);

const { data: changelog } = useAsyncData(
  () => `changelog-${name.value}`,
  () => fetchChangelog(name.value),
  { server: false, default: () => null, watch: [name] },
);

const matchedVersion = computed(
  () => versions.value?.find((v) => v.version === version.value) ?? null,
);

const changelogEntry = computed(() => {
  if (!changelog.value?.entries) return null;
  return changelog.value.entries.find((e) => e.version === version.value) || null;
});

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

useSeoMeta({
  title: () => `${name.value} ${version.value}`,
  description: () => `Version ${version.value} of ${name.value} on pypx.`,
});

defineOgImage(
  "PackageCard",
  {
    name: () => name.value,
    version: () => version.value,
    summary: () => pkg.value?.summary ?? undefined,
    license: () => pkg.value?.license ?? undefined,
  },
  { width: 1200, height: 630 },
);
</script>

<template>
  <div>
    <!-- Back link -->
    <div class="mb-6">
      <NuxtLink
        :to="`/packages/${name}`"
        class="text-sm text-muted transition-colors hover:text-zinc-700 dark:hover:text-zinc-300"
      >
        ← {{ name }}
      </NuxtLink>
    </div>

    <!-- Version not found -->
    <div v-if="versions && !matchedVersion" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Version not found</p>
      <p class="mt-1 text-sm text-muted">No version "{{ version }}" found for "{{ name }}".</p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="matchedVersion">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex items-baseline gap-3">
          <h1 class="text-3xl font-bold text-primary">{{ name }}</h1>
          <span class="rounded bg-raised px-2 py-0.5 font-mono text-sm text-muted">
            {{ version }}
          </span>
        </div>
        <p v-if="pkg?.summary" class="mt-2 text-muted">{{ pkg.summary }}</p>
      </div>

      <!-- Install command -->
      <div class="mb-6">
        <InstallCommand :package-name="`${name}==${version}`" />
      </div>

      <!-- Info grid -->
      <div class="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-muted">Released</p>
          <p class="mt-1 text-sm text-zinc-800 dark:text-zinc-200">
            {{ formatDate(matchedVersion.upload_time) }}
          </p>
        </div>
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-muted">Install Size</p>
          <p class="mt-1 font-mono text-sm text-[var(--color-brand)]">
            {{ formatSize(matchedVersion.install_size) }}
          </p>
        </div>
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-muted">Format</p>
          <p class="mt-1 font-mono text-sm text-zinc-800 dark:text-zinc-200">
            {{ matchedVersion.module_format || "—" }}
          </p>
        </div>
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-muted">Files</p>
          <p class="mt-1 text-sm text-zinc-800 dark:text-zinc-200">
            {{ matchedVersion.files?.length ?? 0 }}
          </p>
        </div>
      </div>

      <!-- Distribution files -->
      <div v-if="matchedVersion.files?.length" class="rounded-lg border border-subtle bg-surface">
        <div class="border-b border-subtle px-4 py-3">
          <h2 class="text-sm font-medium text-zinc-700 dark:text-zinc-300">Distribution Files</h2>
        </div>
        <ul class="divide-y divide-subtle">
          <li
            v-for="file in matchedVersion.files"
            :key="file.filename"
            class="flex items-center justify-between px-4 py-3"
          >
            <span class="font-mono text-sm text-zinc-700 dark:text-zinc-300">{{
              file.filename
            }}</span>
            <span class="ml-4 shrink-0 font-mono text-xs text-[var(--color-brand)]">{{
              formatSize(file.size)
            }}</span>
          </li>
        </ul>
      </div>

      <!-- Changelog entry -->
      <div v-if="changelogEntry" class="mt-6 rounded-lg border border-subtle bg-surface p-6">
        <h2 class="mb-1 text-base font-semibold text-zinc-900 dark:text-zinc-100">
          {{ changelogEntry.title }}
        </h2>
        <div class="mb-4 flex items-center gap-3 text-sm text-muted">
          <span>{{ formatDate(changelogEntry.published_at) }}</span>
          <a
            v-if="changelogEntry.url"
            :href="changelogEntry.url"
            target="_blank"
            rel="noopener noreferrer"
            class="transition-colors hover:text-zinc-700 dark:hover:text-zinc-300"
            >GitHub Release ↗</a
          >
        </div>
        <div class="prose prose-sm max-w-none">
          <div v-if="changelogEntry.body_html" v-html="changelogEntry.body_html" />
          <div v-else class="whitespace-pre-wrap">{{ changelogEntry.body }}</div>
        </div>
      </div>
    </div>

    <!-- Loading state -->
    <div v-else class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>
  </div>
</template>
