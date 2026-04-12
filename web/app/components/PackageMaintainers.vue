<script setup lang="ts">
import type { Maintainer, RepoInfo } from "~/types/api";

const props = defineProps<{
  maintainers: Maintainer[];
  repoInfo?: RepoInfo | null;
}>();

// Build display list: prefer GitHub org owner if available, then maintainers.
const displayMaintainers = computed(() => {
  return props.maintainers.slice(0, 5); // cap at 5
});

const hasAny = computed(() => displayMaintainers.value.length > 0 || !!props.repoInfo?.owner);
</script>

<template>
  <div v-if="hasAny">
    <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-2">Maintainers</div>

    <!-- GitHub org/owner badge -->
    <div v-if="repoInfo?.owner" class="mb-2">
      <a
        :href="repoInfo.owner.url"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-sm text-neutral-300 hover:text-white transition-colors"
      >
        <img
          :src="repoInfo.owner.avatar_url"
          :alt="repoInfo.owner.login"
          class="w-5 h-5 rounded-full"
        />
        <span class="font-medium">
          {{ repoInfo.owner.display_name || repoInfo.owner.login }}
        </span>
        <span
          v-if="repoInfo.owner.is_org"
          class="text-xs px-1.5 py-0.5 rounded bg-neutral-800 text-neutral-400 ring-1 ring-neutral-700"
        >
          org
        </span>
      </a>
    </div>

    <!-- Individual maintainers -->
    <ul v-if="displayMaintainers.length" class="space-y-1">
      <li v-for="m in displayMaintainers" :key="m.email || m.name" class="text-sm text-neutral-400">
        <span v-if="m.name" class="text-neutral-300">{{ m.name }}</span>
        <span v-if="m.name && m.email" class="text-neutral-600"> · </span>
        <a
          v-if="m.email"
          :href="`mailto:${m.email}`"
          class="text-neutral-500 hover:text-neutral-300 transition-colors"
        >
          {{ m.email }}
        </a>
      </li>
    </ul>
  </div>
</template>
