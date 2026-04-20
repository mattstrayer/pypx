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

const allLinks = computed(() => {
  const links = projectLinks.value.slice();
  if (props.pkg.doc_url && !links.some((l) => l.url === props.pkg.doc_url)) {
    links.unshift({ label: "Documentation", url: props.pkg.doc_url });
  }
  return links;
});
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
    <div class="space-y-3">
      <!-- Metadata card -->
      <div class="rounded-lg border border-subtle bg-surface p-4">
        <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">
          Details
        </h2>
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

      <!-- Project links card (includes doc_url as first entry if present) -->
      <div v-if="allLinks.length > 0" class="rounded-lg border border-subtle bg-surface p-4">
        <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">Links</h2>
        <ul class="space-y-1.5 text-sm">
          <li v-for="link in allLinks" :key="link.url">
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

      <!-- GitHub card -->
      <div v-if="repoInfo" class="rounded-lg border border-subtle bg-surface p-4">
        <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">
          GitHub
        </h2>
        <div class="flex gap-4">
          <div v-if="repoInfo.stars" class="flex flex-col gap-0.5">
            <span class="text-base font-semibold text-primary">{{
              repoInfo.stars.toLocaleString()
            }}</span>
            <span class="text-xs text-muted">stars</span>
          </div>
          <div v-if="repoInfo.forks" class="flex flex-col gap-0.5">
            <span class="text-base font-semibold text-primary">{{
              repoInfo.forks.toLocaleString()
            }}</span>
            <span class="text-xs text-muted">forks</span>
          </div>
          <div v-if="repoInfo.open_issues !== undefined" class="flex flex-col gap-0.5">
            <span class="text-base font-semibold text-primary">{{
              repoInfo.open_issues.toLocaleString()
            }}</span>
            <span class="text-xs text-muted">open issues</span>
          </div>
        </div>
        <div v-if="lastPushedAgo" class="mt-2 text-xs text-muted">
          last commit {{ lastPushedAgo }}
        </div>
      </div>

      <!-- Release cadence card -->
      <div
        v-if="pkg.release_cadence?.releases_last_12mo > 0"
        class="rounded-lg border border-subtle bg-surface p-4"
      >
        <h2 class="mb-2 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">
          Release Cadence
        </h2>
        <div class="text-xl font-bold text-primary">
          {{ pkg.release_cadence.releases_last_12mo }}
        </div>
        <div class="text-xs text-muted">releases in the past year</div>
        <div
          v-if="pkg.release_cadence.avg_days_between_releases > 0"
          class="mt-0.5 text-xs text-muted"
        >
          avg {{ Math.round(pkg.release_cadence.avg_days_between_releases) }} days between releases
        </div>
      </div>

      <!-- Platform coverage card — only shown when binary wheels exist (pure-python packages have no platform pills) -->
      <div
        v-if="
          pkg.platform_coverage.linux_x86_64 ||
          pkg.platform_coverage.linux_arm64 ||
          pkg.platform_coverage.macos_x86_64 ||
          pkg.platform_coverage.macos_arm64 ||
          pkg.platform_coverage.windows_x86_64 ||
          pkg.platform_coverage.musl
        "
        class="rounded-lg border border-subtle bg-surface p-4"
      >
        <PackagePlatforms :coverage="pkg.platform_coverage" />
      </div>

      <!-- Maintainers card -->
      <div class="rounded-lg border border-subtle bg-surface p-4">
        <PackageMaintainers :maintainers="pkg.maintainers" :repo-info="repoInfo" />
      </div>
    </div>
  </div>
</template>
