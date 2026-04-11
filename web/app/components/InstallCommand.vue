<script setup lang="ts">
import { useClipboard } from "@vueuse/core";

const props = defineProps<{
  packageName: string;
}>();

const { activeManager, getInstallCommand } = usePackageManager();
const managers = ["uv", "pip", "poetry", "pipx"] as const;

const command = computed(() => getInstallCommand(props.packageName));

const { copy, copied } = useClipboard({ source: command });
</script>

<template>
  <div class="rounded-lg border border-zinc-800 bg-zinc-900">
    <!-- Tab row -->
    <div class="flex gap-1 border-b border-zinc-800 px-3 pt-2">
      <button
        v-for="mgr in managers"
        :key="mgr"
        class="rounded-t px-3 py-1.5 text-xs font-medium transition-colors"
        :class="
          activeManager === mgr
            ? 'bg-[var(--color-brand-muted)] text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]'
            : 'text-zinc-500 hover:text-zinc-300'
        "
        @click="activeManager = mgr"
      >
        {{ mgr }}
      </button>
    </div>

    <!-- Command line -->
    <div class="flex items-center justify-between gap-3 px-4 py-3">
      <code class="font-mono text-sm text-zinc-300">
        <span class="text-zinc-500">$ </span>{{ command }}
      </code>
      <button
        class="shrink-0 rounded p-1.5 text-zinc-500 transition-colors hover:bg-[var(--color-brand-muted)] hover:text-[var(--color-brand)]"
        :title="copied ? 'Copied!' : 'Copy'"
        @click="copy()"
      >
        <svg
          v-if="copied"
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-[var(--color-brand)]"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <polyline points="20 6 9 17 4 12" />
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
      </button>
    </div>
  </div>
</template>
