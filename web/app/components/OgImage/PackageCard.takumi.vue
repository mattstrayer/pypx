<script setup lang="ts">
defineProps<{
  name: string;
  version: string;
  summary?: string;
  license?: string;
}>();
</script>

<template>
  <div
    style="
      width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      background: #09090b;
      padding: 52px 60px;
      position: relative;
      box-sizing: border-box;
    "
  >
    <!-- Grid background -->
    <div
      style="
        position: absolute;
        inset: 0;
        background-image:
          linear-gradient(rgba(74, 222, 128, 0.04) 1px, transparent 1px),
          linear-gradient(90deg, rgba(74, 222, 128, 0.04) 1px, transparent 1px);
        background-size: 32px 32px;
      "
    ></div>
    <!-- Ambient glow top-right -->
    <div
      style="
        position: absolute;
        top: -80px;
        right: -80px;
        width: 320px;
        height: 320px;
        background: radial-gradient(circle, rgba(74, 222, 128, 0.15), transparent 70%);
        border-radius: 50%;
      "
    ></div>
    <!-- Top section: name + version + meta + summary -->
    <div style="position: relative; display: flex; flex-direction: column; gap: 12px">
      <div style="display: flex; align-items: baseline; gap: 14px; flex-wrap: wrap">
        <span
          style="
            font-size: 54px;
            font-weight: 700;
            color: #f4f4f5;
            line-height: 1;
            letter-spacing: -1.5px;
          "
          >{{ name }}</span
        >
        <span
          style="
            font-size: 16px;
            color: #4ade80;
            background: rgba(74, 222, 128, 0.1);
            border: 1px solid rgba(74, 222, 128, 0.25);
            padding: 4px 12px;
            border-radius: 4px;
            font-family: monospace;
            flex-shrink: 0;
          "
          >{{ version }}</span
        >
      </div>
      <div v-if="license" style="font-size: 14px; color: #71717a">{{ license }}</div>
      <p
        v-if="summary"
        style="font-size: 19px; color: #a1a1aa; line-height: 1.45; max-width: 720px; margin: 0"
      >
        {{ summary }}
      </p>
    </div>
    <!-- Footer -->
    <div
      style="position: relative; display: flex; justify-content: space-between; align-items: center"
    >
      <span style="font-size: 14px; color: #3f3f46"
        >pypx.app — The Python Package Index, reimagined</span
      >
      <span style="font-size: 22px; font-weight: 700; color: #4ade80; letter-spacing: -0.5px"
        >pypx</span
      >
    </div>
  </div>
</template>
