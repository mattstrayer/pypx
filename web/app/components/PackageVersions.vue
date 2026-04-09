<script setup lang="ts">
const props = defineProps<{
  name: string
}>()

const { fetchVersions } = useApi()
const { data: versions, status } = await useAsyncData(
  `versions-${props.name}`,
  () => fetchVersions(props.name),
)

const sortedVersions = computed(() => {
  if (!versions.value) return []
  return [...versions.value].sort(
    (a, b) => new Date(b.upload_time).getTime() - new Date(a.upload_time).getTime(),
  )
})

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
        <tr
          v-for="v in sortedVersions"
          :key="v.version"
          class="border-b border-zinc-800 last:border-0"
        >
          <td class="py-3 pr-6">
            <NuxtLink
              :to="`/packages/${name}/${v.version}`"
              class="font-mono hover:text-indigo-400 text-zinc-200 transition-colors"
            >
              {{ v.version }}
            </NuxtLink>
          </td>
          <td class="py-3 pr-6 text-zinc-400">{{ formatDate(v.upload_time) }}</td>
          <td class="py-3 pr-6 font-mono text-emerald-400">{{ formatSize(v.install_size) }}</td>
          <td class="py-3 font-mono text-xs text-zinc-500">{{ v.module_format || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
