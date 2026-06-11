import { describe, it, expect } from "vitest";
import { usePackageManager } from "../usePackageManager";

describe("usePackageManager", () => {
  it("defaults to uv", () => {
    const { activeManager } = usePackageManager();
    expect(activeManager.value).toBe("uv");
  });

  it("generates pip install command", () => {
    const { activeManager, getInstallCommand } = usePackageManager();
    activeManager.value = "pip";
    expect(getInstallCommand("requests")).toBe("pip install requests");
  });

  it("generates uv add command", () => {
    const { activeManager, getInstallCommand } = usePackageManager();
    activeManager.value = "uv";
    expect(getInstallCommand("requests")).toBe("uv add requests");
  });

  it("generates poetry add command", () => {
    const { activeManager, getInstallCommand } = usePackageManager();
    activeManager.value = "poetry";
    expect(getInstallCommand("requests")).toBe("poetry add requests");
  });

  it("generates pipx install command", () => {
    const { activeManager, getInstallCommand } = usePackageManager();
    activeManager.value = "pipx";
    expect(getInstallCommand("requests")).toBe("pipx install requests");
  });

  it("switching activeManager.value changes getInstallCommand output", () => {
    const { activeManager, getInstallCommand } = usePackageManager();
    activeManager.value = "pip";
    expect(getInstallCommand("numpy")).toBe("pip install numpy");
    activeManager.value = "uv";
    expect(getInstallCommand("numpy")).toBe("uv add numpy");
  });

  it("state is shared across two calls to usePackageManager in the same test", () => {
    const a = usePackageManager();
    const b = usePackageManager();
    a.activeManager.value = "poetry";
    expect(b.activeManager.value).toBe("poetry");
    expect(b.getInstallCommand("flask")).toBe("poetry add flask");
  });
});
