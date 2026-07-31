<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    keywords?: string[];
    sticky?: boolean;
    placementId?: string;
  }>(),
  { keywords: () => [], sticky: false, placementId: undefined },
);

const { enabled, publisher, type, reload } = useEthicalAds();

const filled = ref(false);
const placementEl = ref<HTMLElement | null>(null);
let observer: MutationObserver | undefined;

const keywordAttr = computed(() =>
  props.keywords.length > 0 ? props.keywords.join("|") : undefined,
);

const cardClass = computed(() => {
  if (!filled.value) return "";
  const base = "rounded-lg border border-subtle bg-surface p-4";
  return props.sticky ? `sticky top-20 ${base}` : base;
});

function syncFilled(): void {
  filled.value = (placementEl.value?.children.length ?? 0) > 0;
}

onMounted(() => {
  const el = placementEl.value;
  if (!el) return;
  syncFilled();
  observer = new MutationObserver(syncFilled);
  observer.observe(el, { childList: true });

  // The vendor client only scans for placements when its script executes. A slot
  // mounted after that — crossing the 1024px breakpoint swaps one AdSlot for the
  // other, and the script tag is app-scoped so it never re-executes — would stay
  // permanently and silently empty. `reload()` no-ops while the client is absent,
  // which is exactly right: in that case its own initial scan finds this slot.
  reload();
});

onUnmounted(() => {
  observer?.disconnect();
});

// Package-to-package navigation keeps this component mounted; without an
// explicit reload the ad would stay pinned to the first package's keywords.
// The MutationObserver reports the resulting fill state on its own once the
// vendor client injects (or clears) content, so we don't await anything here.
//
// `flush: "post"` is load-bearing. The vendor's reload() synchronously re-reads
// the keywords straight off the DOM attribute, so a default "pre" watcher would
// fire before Vue patches `data-ea-keywords` and request an ad targeted at the
// PREVIOUS package — leaving every later navigation one package behind.
watch(
  () => keywordAttr.value,
  (next, previous) => {
    if (next === previous) return;
    filled.value = false;
    reload();
  },
  { flush: "post" },
);
</script>

<template>
  <div v-if="enabled" :class="cardClass">
    <h2 v-if="filled" class="section-label">Sponsored</h2>
    <div
      ref="placementEl"
      :id="placementId"
      class="pypx-ad adaptive-css"
      :data-ea-publisher="publisher"
      :data-ea-type="type"
      :data-ea-keywords="keywordAttr"
    />
  </div>
</template>
