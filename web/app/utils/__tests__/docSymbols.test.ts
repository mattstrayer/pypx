import { describe, it, expect } from "vitest";
import { docEntries } from "../docSymbols";
import type { DocModule, DocSymbol } from "~/types/api";

function sym(name: string, kind: DocSymbol["kind"] = "function"): DocSymbol {
  return { name, kind, signature: "", docstring: "", parameters: [] };
}

function mod(name: string, parts: Partial<DocModule> = {}): DocModule {
  return { name, functions: [], classes: [], exceptions: [], ...parts };
}

describe("docEntries", () => {
  it("keeps same-named symbols from different modules, in module order", () => {
    const { classes } = docEntries([
      mod("redis.asyncio.client", { classes: [sym("Redis", "class")] }),
      mod("redis.client", { classes: [sym("Redis", "class"), sym("Pipeline", "class")] }),
    ]);
    expect(classes.map((c) => c.id)).toEqual([
      "redis.asyncio.client.Redis",
      "redis.client.Redis",
      "redis.client.Pipeline",
    ]);
    expect(classes.map((c) => c.repeated)).toEqual([true, true, false]);
    expect(classes[1]!.module).toBe("redis.client");
  });

  it("gives a function and a class with one name their own paths", () => {
    const e = docEntries([
      mod("fastapi.param_functions", { functions: [sym("Query")] }),
      mod("fastapi.params", { classes: [sym("Query", "class")] }),
    ]);
    expect(e.functions[0]!.id).not.toBe(e.classes[0]!.id);
    expect(e.functions[0]!.repeated).toBe(true);
    expect(e.classes[0]!.repeated).toBe(true);
  });

  it("keeps ids unique when two symbols share a dotted path", () => {
    const e = docEntries([
      mod("m", {
        functions: [sym("x"), sym("x")],
        classes: [sym("x", "class")],
      }),
    ]);
    const ids = [...e.functions, ...e.classes].map((s) => s.id);
    expect(new Set(ids).size).toBe(3);
    expect(e.functions[0]!.id).toBe("m.x");
    expect(e.functions[1]!.path).toBe("m.x");
  });

  it("handles no modules", () => {
    expect(docEntries([])).toEqual({ functions: [], classes: [], exceptions: [] });
  });
});
