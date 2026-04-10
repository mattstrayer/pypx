type PackageManager = "pip" | "uv" | "poetry" | "pipx";

export function usePackageManager() {
  const activeManager = useState<PackageManager>("package-manager", () => "pip");

  function getInstallCommand(packageName: string): string {
    switch (activeManager.value) {
      case "pip":
        return `pip install ${packageName}`;
      case "uv":
        return `uv add ${packageName}`;
      case "poetry":
        return `poetry add ${packageName}`;
      case "pipx":
        return `pipx install ${packageName}`;
    }
  }
  return { activeManager, getInstallCommand };
}
