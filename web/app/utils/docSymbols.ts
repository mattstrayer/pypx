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
  /** Dotted paths of the aliases that point at this symbol. */
  importableAs: string[];
}

/** An alias whose target has a card on the docs page. */
export interface DocAliasEntry {
  name: string;
  module: string;
  /** Dotted path of the alias, such as "numpy.strings.capitalize". */
  path: string;
  /** The id of the target's card. */
  targetId: string;
}

export interface DocEntries {
  functions: DocEntry[];
  classes: DocEntry[];
  exceptions: DocEntry[];
  aliases: DocAliasEntry[];
}

/**
 * Lists every symbol, in module order. Two symbols with one name are different
 * definitions, because the API publishes each definition once, so all of them stay.
 * An alias whose target has no card, such as a module attribute, is not in `aliases`.
 */
export function docEntries(modules: DocModule[]): DocEntries {
  const nameCounts = new Map<string, number>();
  for (const m of modules) {
    for (const s of [...m.functions, ...m.classes, ...m.exceptions]) {
      nameCounts.set(s.name, (nameCounts.get(s.name) ?? 0) + 1);
    }
  }

  const importableAs = new Map<string, string[]>();
  for (const m of modules) {
    for (const a of m.aliases ?? []) {
      const paths = importableAs.get(a.target) ?? [];
      paths.push(`${m.name}.${a.name}`);
      importableAs.set(a.target, paths);
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
      importableAs: importableAs.get(path) ?? [],
    };
  };

  const functions = modules.flatMap((m) => m.functions.map((s) => entry(m.name, s)));
  const classes = modules.flatMap((m) => m.classes.map((s) => entry(m.name, s)));
  const exceptions = modules.flatMap((m) => m.exceptions.map((s) => entry(m.name, s)));

  // Two symbols can share a path. The alias then jumps to the first card.
  const firstIds = new Map<string, string>();
  for (const s of [...functions, ...classes, ...exceptions]) {
    if (!firstIds.has(s.path)) firstIds.set(s.path, s.id);
  }
  const aliases = modules.flatMap((m) =>
    (m.aliases ?? []).flatMap((a) => {
      const targetId = firstIds.get(a.target);
      return targetId
        ? [{ name: a.name, module: m.name, path: `${m.name}.${a.name}`, targetId }]
        : [];
    }),
  );

  return { functions, classes, exceptions, aliases };
}
