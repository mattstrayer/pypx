export function useThemeTransition() {
  function withTransition(fn: () => void) {
    if (!import.meta.client) {
      fn();
      return;
    }

    if (document.startViewTransition) {
      document.startViewTransition(async () => {
        fn();
        await nextTick();
      });
    } else {
      // CSS fallback for Firefox
      const el = document.documentElement;
      el.classList.add("theme-transitioning");
      fn();
      setTimeout(() => el.classList.remove("theme-transitioning"), 350);
    }
  }

  return { withTransition };
}
