<script setup lang="ts">
const route = useRoute()
const name = computed(() => route.params.name as string)
const version = computed(() => route.params.version as string)

const { fetchPackage, fetchVersions } = useApi()

const [{ data: pkg }, { data: versions }] = await Promise.all([
  useAsyncData(`package-${name.value}`, () => fetchPackage(name.value)),
  useAsyncData(`versions-${name.value}`, () => fetchVersions(name.value)),
])

const matchedVersion = computed(() =>
  versions.value?.find(v => v.version === version.value) ?? null,
)

function formatSize(bytes: number): string {
  if (!bytes) return '—'
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${bytes} B`
}

function formatDate(iso: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

useHead({
  title: computed(() => `${name.value} ${version.value} — pypx`),
})
</script>

<template>
  <div>
    <!-- Back link -->
    <div class="mb-6">
      <NuxtLink
        :to="`/packages/${name}`"
        class="text-sm text-zinc-500 transition-colors hover:text-zinc-300"
      >
        ← {{ name }}
      </NuxtLink>
    </div>

    <!-- Version not found -->
    <div v-if="versions && !matchedVersion" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-300">Version not found</p>
      <p class="mt-1 text-sm text-zinc-500">
        No version "{{ version }}" found for "{{ name }}".
      </p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="matchedVersion">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex items-baseline gap-3">
          <h1 class="text-3xl font-bold text-zinc-50">{{ name }}</h1>
          <span class="rounded bg-zinc-800 px-2 py-0.5 font-mono text-sm text-zinc-400">
            {{ version }}
          </span>
        </div>
        <p v-if="pkg?.summary" class="mt-2 text-zinc-400">{{ pkg.summary }}</p>
      </div>

      <!-- Install command -->
      <div class="mb-6">
        <InstallCommand :package-name="`${name}==${version}`" />
      </div>

      <!-- Info grid -->
      <div class="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-zinc-500">Released</p>
          <p class="mt-1 text-sm text-zinc-200">{{ formatDate(matchedVersion.upload_time) }}</p>
        </div>
        <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-zinc-500">Install Size</p>
          <p class="mt-1 font-mono text-sm text-emerald-400">{{ formatSize(matchedVersion.install_size) }}</p>
        </div>
        <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-zinc-500">Format</p>
          <p class="mt-1 font-mono text-sm text-zinc-200">{{ matchedVersion.module_format || '—' }}</p>
        </div>
        <div class="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <p class="text-xs font-medium uppercase tracking-wide text-zinc-500">Files</p>
          <p class="mt-1 text-sm text-zinc-200">{{ matchedVersion.files?.length ?? 0 }}</p>
        </div>
      </div>

      <!-- Distribution files -->
      <div v-if="matchedVersion.files?.length" class="rounded-lg border border-zinc-800 bg-zinc-900">
        <div class="border-b border-zinc-800 px-4 py-3">
          <h2 class="text-sm font-medium text-zinc-300">Distribution Files</h2>
        </div>
        <ul class="divide-y divide-zinc-800">
          <li
            v-for="file in matchedVersion.files"
            :key="file.filename"
            class="flex items-center justify-between px-4 py-3"
          >
            <span class="font-mono text-sm text-zinc-300">{{ file.filename }}</span>
            <span class="ml-4 shrink-0 font-mono text-xs text-emerald-400">{{ formatSize(file.size) }}</span>
          </li>
        </ul>
      </div>
    </div>

    <!-- Loading state -->
    <div v-else class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>
  </div>
</template>
