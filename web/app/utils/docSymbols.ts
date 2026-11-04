import type { DocModule, DocSymbol } from "~/types/api";

/** A symbol on the docs page, with where it lives in the package. */
export interface DocEntry extends DocSymbol {
  module: string;
  /** Dotted path, such as "redis.client.Redis". */
  path: string;
  /** Unique on the page. It is the path, unless two symbols share one. */
  id: string;
  /** True if another symbol in the package has the same bare name. */
  repeated: boolean;
}

export interface DocEntries {
  functions: DocEntry[];
  classes: DocEntry[];
  exceptions: DocEntry[];
}

/**
 * Lists every symbol, in module order. Two symbols with one name are different
 * definitions, because the API publishes each definition once, so all of them stay.
 */
export function docEntries(modules: DocModule[]): DocEntries {
  const nameCounts = new Map<string, number>();
  for (const m of modules) {
    for (const s of [...m.functions, ...m.classes, ...m.exceptions]) {
      nameCounts.set(s.name, (nameCounts.get(s.name) ?? 0) + 1);
    }
  }

  const pathCounts = new Map<string, number>();
  const entry = (module: string, s: DocSymbol): DocEntry => {
    const path = `${module}.${s.name}`;
    const seen = (pathCounts.get(path) ?? 0) + 1;
    pathCounts.set(path, seen);
    return {
      ...s,
      module,
      path,
      id: seen === 1 ? path : `${path}~${seen}`,
      repeated: (nameCounts.get(s.name) ?? 0) > 1,
    };
  };

  return {
    functions: modules.flatMap((m) => m.functions.map((s) => entry(m.name, s))),
    classes: modules.flatMap((m) => m.classes.map((s) => entry(m.name, s))),
    exceptions: modules.flatMap((m) => m.exceptions.map((s) => entry(m.name, s))),
  };
}
