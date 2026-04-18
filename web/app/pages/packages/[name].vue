<script setup lang="ts">
const route = useRoute();
const name = computed(() => route.params.name as string);
const activeTab = ref("overview");
const isChildRoute = computed(() => route.path !== `/packages/${name.value}`);

const api = useApi();
const { data: pkg, status } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

// Non-blocking parallel fetches (client-side, don't block SSR)
const { data: security } = useAsyncData(
  `security-${name.value}`,
  () => api.fetchSecurity(name.value, pkg.value?.version),
  { server: false, default: () => null },
);

const { data: extras } = useAsyncData(`extras-${name.value}`, () => api.fetchExtras(name.value), {
  server: false,
  default: () => null,
});

const { data: changelog } = useAsyncData(
  `changelog-${name.value}`,
  () => api.fetchChangelog(name.value).catch(() => null),
  { server: false, default: () => null },
);

// Docs: non-blocking fetch to determine if the Docs tab should be shown.
const { data: docsData } = useAsyncData(
  `docs-${name.value}`,
  () => api.fetchDocs(name.value).catch(() => null),
  { server: false, default: () => null },
);

const repoInfo = computed(() => extras.value?.repo_info ?? null);

const maintenanceStatus = useMaintenanceStatus(pkg, repoInfo);

const inPageTabs = [
  { key: "overview", label: "Overview", shortcut: "1" },
  { key: "dependencies", label: "Dependencies", shortcut: "2" },
  { key: "versions", label: "Versions", shortcut: "3" },
  { key: "stats", label: "Stats", shortcut: "4" },
];

const { register, unregister } = useKeyboardShortcuts();

onMounted(() => {
  for (const tab of inPageTabs) {
    register(tab.shortcut, () => (activeTab.value = tab.key));
  }
  register("/", () => {
    document.querySelector<HTMLInputElement>('header input[type="text"]')?.focus();
  });
});

onUnmounted(() => {
  for (const tab of inPageTabs) {
    unregister(tab.shortcut);
  }
  unregister("/");
});

useSeoMeta({
  title: () => (pkg.value ? pkg.value.name : "Loading"),
  description: () => pkg.value?.summary || "",
  ogTitle: () => (pkg.value ? `${pkg.value.name} ${pkg.value.version}` : "pypx"),
  ogDescription: () => pkg.value?.summary || "",
});

defineOgImage(
  "PackageCard",
  {
    name: () => pkg.value?.name ?? "",
    version: () => pkg.value?.version ?? "",
    summary: () => pkg.value?.summary ?? undefined,
    license: () => pkg.value?.license ?? undefined,
  },
  { width: 1200, height: 630 },
);

useSchemaOrg(
  computed(() =>
    pkg.value
      ? [
          {
            "@type": "SoftwareApplication" as const,
            name: pkg.value.name,
            description: pkg.value.summary || undefined,
            softwareVersion: pkg.value.version,
            applicationCategory: "DeveloperApplication",
            license: pkg.value.license || undefined,
          },
        ]
      : [],
  ),
);
</script>

<template>
  <!-- Child routes (e.g. /docs) are self-contained — pass straight through -->
  <NuxtPage v-if="isChildRoute" />

  <div v-else>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package not found</p>
      <p class="mt-1 text-sm text-muted">No package named "{{ name }}" could be found.</p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <h1 class="text-3xl font-bold text-primary">{{ pkg.name }}</h1>
          <span class="rounded bg-raised px-2 py-0.5 font-mono text-sm text-muted">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-muted">{{ pkg.summary }}</p>
        <div class="mt-3">
          <PackageBadges
            :pkg="pkg"
            :extras="extras"
            :security="security"
            :maintenance-status="maintenanceStatus"
          />
        </div>
      </div>

      <!-- Tabs -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-subtle pb-0">
        <!-- In-page tabs -->
        <button
          v-for="tab in inPageTabs"
          :key="tab.key"
          class="group cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors border-b-2 border-transparent"
          :class="
            activeTab === tab.key
              ? 'bg-raised text-primary'
              : 'text-zinc-700 dark:text-zinc-300 hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]'
          "
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <KbdHint :keys="tab.shortcut" />
        </button>

        <!-- Docs tab — link to separate route, shown only when available -->
        <NuxtLink
          v-if="docsData?.available"
          :to="`/packages/${pkg.name}/docs`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-zinc-700 dark:text-zinc-300 transition-colors border-b-2 border-transparent hover:border-[rgba(4,120,87,0.65)] dark:hover:border-[rgba(74,222,128,0.65)]"
        >
          Docs
        </NuxtLink>
      </div>

      <!-- Tab content -->
      <div>
        <div v-if="activeTab === 'overview'">
          <PackageOverview :pkg="pkg" :repo-info="repoInfo" />
        </div>
        <div v-else-if="activeTab === 'dependencies'">
          <PackageDependencies :name="pkg.name" :dependencies="pkg.dependencies" />
        </div>
        <div v-else-if="activeTab === 'versions'"><PackageVersions :name="pkg.name" /></div>
        <div v-else-if="activeTab === 'stats'">
          <PackageStats :name="pkg.name" />
        </div>
      </div>
    </div>
  </div>
</template>
