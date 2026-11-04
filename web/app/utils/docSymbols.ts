import type { DocConstant, DocModule, DocSymbol } from "~/types/api";

/**
 * The words shown beside a symbol's name, one per fact, as in `/docs.txt`.
 * `method_kind` alone carries classmethod and staticmethod.
 */
export function docBadges(symbol: Pick<DocSymbol, "method_kind" | "decorators">): string[] {
  const words = (symbol.decorators ?? []).map((d) => (d === "abstractmethod" ? "abstract" : d));
  return symbol.method_kind ? [symbol.method_kind, ...words] : words;
}

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

/** A constant or type alias on the docs page. It has a row, not a card. */
export interface DocConstantEntry extends DocConstant {
  kind: "constant";
  module: string;
  /** Dotted path, such as "requests.adapters.DEFAULT_POOLSIZE". */
  path: string;
  /** Unique on the page, as for a card. */
  id: string;
  /** True if another symbol in the package has the same bare name. */
  repeated: boolean;
}

/** An alias whose target has a card or a constant row on the docs page. */
export interface DocAliasEntry {
  name: string;
  module: string;
  /** Dotted path of the alias, such as "numpy.strings.capitalize". */
  path: string;
  /** The id of the target's card or row. */
  targetId: string;
}

export interface DocEntries {
  functions: DocEntry[];
  classes: DocEntry[];
  exceptions: DocEntry[];
  constants: DocConstantEntry[];
  aliases: DocAliasEntry[];
}

/**
 * Lists every symbol, in module order. Two symbols with one name are different
 * definitions, because the API publishes each definition once, so all of them stay.
 * An alias whose target has no card or row, such as a ufunc, is not in `aliases`.
 */
export function docEntries(modules: DocModule[]): DocEntries {
  const nameCounts = new Map<string, number>();
  for (const m of modules) {
    for (const s of [...m.functions, ...m.classes, ...m.exceptions, ...(m.constants ?? [])]) {
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
  const placed = (module: string, name: string) => {
    const path = `${module}.${name}`;
    const seen = (pathCounts.get(path) ?? 0) + 1;
    pathCounts.set(path, seen);
    return {
      module,
      path,
      id: seen === 1 ? path : `${path}~${seen}`,
      repeated: (nameCounts.get(name) ?? 0) > 1,
    };
  };
  const entry = (module: string, s: DocSymbol): DocEntry => {
    const place = placed(module, s.name);
    return { ...s, ...place, importableAs: importableAs.get(place.path) ?? [] };
  };

  const functions = modules.flatMap((m) => m.functions.map((s) => entry(m.name, s)));
  const classes = modules.flatMap((m) => m.classes.map((s) => entry(m.name, s)));
  const exceptions = modules.flatMap((m) => m.exceptions.map((s) => entry(m.name, s)));
  // These come last, so a constant never changes the id of a card.
  const constants = modules.flatMap((m) =>
    (m.constants ?? []).map((c) => ({
      ...c,
      kind: "constant" as const,
      ...placed(m.name, c.name),
    })),
  );

  // Two symbols can share a path. The alias then jumps to the first one.
  const firstIds = new Map<string, string>();
  for (const s of [...functions, ...classes, ...exceptions, ...constants]) {
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

  return { functions, classes, exceptions, constants, aliases };
}
