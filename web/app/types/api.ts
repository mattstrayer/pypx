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

export interface VersionInfo {
  version: string;
  install_size: number;
  module_format: string;
  upload_time: string;
  files: FileInfo[];
}

export interface StatsData {
  package: string;
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
}
