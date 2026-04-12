export interface PackageData {
  name: string;
  version: string;
  summary: string;
  description: string;
  description_content_type: string;
  description_html: string;
  license: string;
  author: string;
  author_email: string;
  home_page: string;
  requires_python: string;
  requires_dist: string[];
  project_urls: Record<string, string>;
  classifiers: string[];
  latest_files: FileInfo[];
  install_size: number;
  module_format: string;
  python_versions: PythonVersionInfo;
  dependencies: DependencyTree;
  platform_coverage: PlatformCoverage;
  release_cadence: ReleaseCadence;
  maintainers: Maintainer[];
  doc_url: string;
}

export interface FileInfo {
  filename: string;
  size: number;
  package_type: string;
  python_version: string;
  upload_time: string;
}

export interface PythonVersionInfo {
  constraint: string;
  min_version: string;
}

export interface Dependency {
  name: string;
  constraint: string;
}

export interface DependencyTree {
  required: Dependency[];
  extras: Record<string, Dependency[]>;
}

export interface PlatformCoverage {
  pure_python: boolean;
  linux_x86_64: boolean;
  linux_arm64: boolean;
  macos_x86_64: boolean;
  macos_arm64: boolean;
  windows_x86_64: boolean;
  musl: boolean;
}

export interface QuarterCount {
  quarter: string;
  count: number;
}

export interface ReleaseCadence {
  releases_last_12mo: number;
  avg_days_between_releases: number;
  last_released_at: string;
  quarterly_counts: QuarterCount[];
}

export interface Maintainer {
  name: string;
  email: string;
}

export interface VersionInfo {
  version: string;
  install_size: number;
  module_format: string;
  upload_time: string;
  files: FileInfo[];
}

export interface StatsData {
  package: string;
  period: string;
  date_range?: { from: string; to: string };
  overall: DataPoint[];
  python_versions: DataPoint[];
  systems: DataPoint[];
}

export interface DataPoint {
  category: string;
  downloads: number;
}

export interface SearchResult {
  name: string;
  summary: string;
  downloads: number;
}

export interface RepoOwner {
  login: string;
  avatar_url: string;
  display_name: string;
  url: string;
  is_org: boolean;
}

export interface RepoInfo {
  stars: number;
  forks: number;
  open_issues: number;
  last_pushed_at: string;
  owner: RepoOwner;
}

export interface ChangelogEntry {
  version: string;
  tag_name: string;
  title: string;
  body: string;
  body_html: string;
  published_at: string;
  url: string;
}

export interface ChangelogData {
  package: string;
  source: string;
  repo_url: string;
  entries: ChangelogEntry[];
  repo_info?: RepoInfo;
}

export interface VulnInfo {
  id: string;
  summary: string;
  severity: string;
  affected_range: string;
  fixed_in?: string;
  url: string;
}

export interface SecurityData {
  package: string;
  vulns: VulnInfo[];
  checked_at: string;
}

export interface TypeSupport {
  status: "typed" | "stubs" | "untyped";
  stubs_package?: string;
}

export interface CondaForgeInfo {
  available: boolean;
  version?: string;
  url?: string;
}

export interface ExtrasData {
  package: string;
  type_support: TypeSupport;
  conda_forge?: CondaForgeInfo;
}
