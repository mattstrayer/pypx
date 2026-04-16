<script setup lang="ts">
import { useTimeAgo } from "@vueuse/core";
import type { PackageData, RepoInfo } from "~/types/api";

const props = defineProps<{
  pkg: PackageData;
  repoInfo?: RepoInfo | null;
}>();

const maintainer = computed(() => props.pkg.author || props.pkg.author_email || null);

const projectLinks = computed(() => {
  const urls = props.pkg.project_urls;
  if (!urls) return [];
  return Object.entries(urls).map(([label, url]) => ({ label, url }));
});

const lastPushedAgo = computed(() =>
  props.repoInfo?.last_pushed_at ? useTimeAgo(new Date(props.repoInfo.last_pushed_at)).value : null,
);
</script>

<template>
  <div class="grid gap-6 lg:grid-cols-[1fr_300px]">
    <!-- Main column -->
    <div class="min-w-0 space-y-6">
      <!-- Install command -->
      <InstallCommand :package-name="pkg.name" />

      <!-- Description -->
      <div
        v-if="pkg.description"
        class="overflow-hidden rounded-lg border border-subtle bg-surface p-5"
      >
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted">Description</h2>
        <div class="flex gap-6">
          <div class="min-w-0 flex-1">
            <div
              v-if="pkg.description_html"
              id="readme-content"
              class="prose prose-sm max-w-none"
              v-html="pkg.description_html"
            />
            <div
              v-else
              class="whitespace-pre-wrap break-words text-sm leading-relaxed text-zinc-700 dark:text-zinc-300"
            >
              {{ pkg.description }}
            </div>
          </div>
          <div
            v-if="pkg.description_html"
            class="sticky top-20 hidden w-48 shrink-0 self-start xl:block"
          >
            <ReadmeOutline container-selector="#readme-content" />
          </div>
        </div>
      </div>
    </div>

    <!-- Sidebar -->
    <div class="space-y-4">
      <!-- Metadata card -->
      <div class="rounded-lg border border-subtle bg-surface p-4">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted">Details</h2>
        <dl class="space-y-2 text-sm">
          <div class="flex justify-between gap-2">
            <dt class="text-muted">Version</dt>
            <dd class="font-mono text-zinc-700 dark:text-zinc-300">{{ pkg.version }}</dd>
          </div>
          <div v-if="pkg.license" class="flex justify-between gap-2">
            <dt class="text-muted">License</dt>
            <dd class="text-right text-zinc-700 dark:text-zinc-300">{{ pkg.license }}</dd>
          </div>
          <div v-if="pkg.requires_python" class="flex justify-between gap-2">
            <dt class="text-muted">Python</dt>
            <dd class="font-mono text-zinc-700 dark:text-zinc-300">{{ pkg.requires_python }}</dd>
          </div>
          <div v-if="maintainer" class="flex justify-between gap-2">
            <dt class="text-muted">Maintainer</dt>
            <dd class="truncate text-right text-zinc-700 dark:text-zinc-300">{{ maintainer }}</dd>
          </div>
        </dl>
      </div>

      <!-- Project links card -->
      <div v-if="projectLinks.length > 0" class="rounded-lg border border-subtle bg-surface p-4">
        <h2 class="mb-3 text-sm font-semibold uppercase tracking-wider text-muted">Links</h2>
        <ul class="space-y-1.5 text-sm">
          <li v-for="link in projectLinks" :key="link.label">
            <a
              :href="link.url"
              target="_blank"
              rel="noopener noreferrer"
              class="flex items-center gap-1.5 text-muted transition-colors hover:text-zinc-800 dark:hover:text-zinc-200"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-3.5 w-3.5 shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
              {{ link.label }}
            </a>
          </li>
        </ul>
      </div>

      <!-- GitHub health signals -->
      <div v-if="repoInfo" class="pt-3 border-t border-neutral-800">
        <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-2">GitHub</div>
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-neutral-400">
          <span v-if="repoInfo.stars">
            <span class="text-neutral-300">{{ repoInfo.stars.toLocaleString() }}</span> stars
          </span>
          <span v-if="repoInfo.forks">
            <span class="text-neutral-300">{{ repoInfo.forks.toLocaleString() }}</span> forks
          </span>
          <span v-if="repoInfo.open_issues !== undefined">
            <span class="text-neutral-300">{{ repoInfo.open_issues.toLocaleString() }}</span> open
            issues
          </span>
        </div>
        <div v-if="lastPushedAgo" class="text-xs text-neutral-500 mt-1">
          last commit {{ lastPushedAgo }}
        </div>
      </div>

      <!-- Doc link button -->
      <div v-if="pkg.doc_url" class="pt-3 border-t border-neutral-800">
        <a
          :href="pkg.doc_url"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 text-sm text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
        >
          Documentation →
        </a>
      </div>

      <!-- Release cadence -->
      <div
        v-if="pkg.release_cadence?.releases_last_12mo > 0"
        class="pt-3 border-t border-neutral-800"
      >
        <div class="text-xs font-medium text-neutral-400 uppercase tracking-wide mb-1">
          Release Cadence
        </div>
        <div class="text-sm text-neutral-400">
          <span class="text-neutral-300">{{ pkg.release_cadence.releases_last_12mo }}</span>
          releases in the past year
          <span v-if="pkg.release_cadence.avg_days_between_releases > 0">
            · avg {{ Math.round(pkg.release_cadence.avg_days_between_releases) }} days apart
          </span>
        </div>
      </div>

      <!-- Platform coverage -->
      <div class="pt-3 border-t border-neutral-800">
        <PackagePlatforms :coverage="pkg.platform_coverage" />
      </div>

      <!-- Maintainers -->
      <div class="pt-3 border-t border-neutral-800">
        <PackageMaintainers :maintainers="pkg.maintainers" :repo-info="repoInfo" />
      </div>
    </div>
  </div>
</template>
