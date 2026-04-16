type ShortcutHandler = () => void;

export function useKeyboardShortcuts() {
  const shortcuts = new Map<string, ShortcutHandler>();

  function handleKeydown(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement)?.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
    if (e.metaKey || e.ctrlKey || e.altKey) return;

    const handler = shortcuts.get(e.key);
    if (handler) {
      e.preventDefault();
      handler();
    }
  }

  function register(key: string, handler: ShortcutHandler) {
    shortcuts.set(key, handler);
  }

  function unregister(key: string) {
    shortcuts.delete(key);
  }

  if (import.meta.client) {
    onMounted(() => window.addEventListener("keydown", handleKeydown));
    onUnmounted(() => {
      window.removeEventListener("keydown", handleKeydown);
      shortcuts.clear();
    });
  }

  return { register, unregister };
}
