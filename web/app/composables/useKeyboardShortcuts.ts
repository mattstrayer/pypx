type ShortcutHandler = () => void;

const shortcuts = new Map<string, ShortcutHandler>();
let initialized = false;

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

export function useKeyboardShortcuts() {
  function register(key: string, handler: ShortcutHandler) {
    shortcuts.set(key, handler);
  }

  function unregister(key: string) {
    shortcuts.delete(key);
  }

  if (import.meta.client && !initialized) {
    initialized = true;
    window.addEventListener("keydown", handleKeydown);
  }

  return { register, unregister };
}
