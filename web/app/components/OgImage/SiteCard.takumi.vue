<template>
  <div
    style="
      width: 100%;
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      background: #09090b;
      position: relative;
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
        width: 360px;
        height: 360px;
        background: radial-gradient(circle, rgba(74, 222, 128, 0.18), transparent 70%);
        border-radius: 50%;
      "
    ></div>
    <!-- Content -->
    <div
      style="
        position: relative;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 16px;
      "
    >
      <span
        style="
          font-size: 80px;
          font-weight: 700;
          color: #4ade80;
          letter-spacing: -3px;
          line-height: 1;
        "
        >pypx</span
      >
      <span
        style="
          font-size: 24px;
          color: #a1a1aa;
          text-align: center;
          max-width: 600px;
          line-height: 1.4;
        "
        >The Python Package Index, reimagined</span
      >
      <span style="font-size: 15px; color: #52525b; margin-top: 12px">pypx.app</span>
    </div>
  </div>
</template>
