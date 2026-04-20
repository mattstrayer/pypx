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
    <h2 class="mb-2.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">
      Maintainers
    </h2>

    <!-- GitHub org/owner badge -->
    <div v-if="repoInfo?.owner" class="mb-2">
      <a
        :href="repoInfo.owner.url"
        target="_blank"
        rel="noopener noreferrer"
        class="inline-flex items-center gap-2 text-sm text-zinc-700 transition-colors hover:text-primary dark:text-zinc-300"
      >
        <img
          :src="repoInfo.owner.avatar_url"
          :alt="repoInfo.owner.login"
          class="h-5 w-5 rounded-full"
        />
        <span class="font-medium">
          {{ repoInfo.owner.display_name || repoInfo.owner.login }}
        </span>
        <span
          v-if="repoInfo.owner.is_org"
          class="rounded bg-raised px-1.5 py-0.5 text-xs text-muted ring-1 ring-subtle"
        >
          org
        </span>
      </a>
    </div>

    <!-- Individual maintainers -->
    <ul v-if="displayMaintainers.length" class="space-y-1">
      <li v-for="m in displayMaintainers" :key="m.email || m.name" class="text-sm text-muted">
        <span v-if="m.name" class="text-zinc-700 dark:text-zinc-300">{{ m.name }}</span>
        <span v-if="m.name && m.email" class="text-muted"> · </span>
        <a
          v-if="m.email"
          :href="`mailto:${m.email}`"
          class="text-muted transition-colors hover:text-primary"
        >
          {{ m.email }}
        </a>
      </li>
    </ul>
  </div>
</template>
