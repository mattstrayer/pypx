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
    expect(docEntries([])).toEqual({
      functions: [],
      classes: [],
      exceptions: [],
      constants: [],
      aliases: [],
    });
  });

  it("lists the aliases of a symbol on its entry", () => {
    const e = docEntries([
      mod("numpy", { aliases: [{ name: "cap", target: "numpy.char.capitalize" }] }),
      mod("numpy.char", { functions: [sym("capitalize"), sym("lower")] }),
      mod("numpy.strings", { aliases: [{ name: "capitalize", target: "numpy.char.capitalize" }] }),
    ]);
    expect(e.functions[0]!.importableAs).toEqual(["numpy.cap", "numpy.strings.capitalize"]);
    expect(e.functions[1]!.importableAs).toEqual([]);
  });

  it("keeps only the aliases with a card as jump targets", () => {
    const e = docEntries([
      mod("numpy.char", { functions: [sym("capitalize")] }),
      mod("numpy.strings", {
        aliases: [
          { name: "capitalize", target: "numpy.char.capitalize" },
          { name: "isalnum", target: "numpy.char.isalnum" },
        ],
      }),
    ]);
    expect(e.aliases).toEqual([
      {
        name: "capitalize",
        module: "numpy.strings",
        path: "numpy.strings.capitalize",
        targetId: "numpy.char.capitalize",
      },
    ]);
  });

  it("jumps an alias to the first of two symbols with one path", () => {
    const e = docEntries([
      mod("m", { functions: [sym("x")], classes: [sym("x", "class")] }),
      mod("n", { aliases: [{ name: "x", target: "m.x" }] }),
    ]);
    expect(e.aliases[0]!.targetId).toBe(e.functions[0]!.id);
    expect(e.classes[0]!.importableAs).toEqual(["n.x"]);
  });

  describe("constants", () => {
    it("lists constants in module order with a path and an id", () => {
      const { constants } = docEntries([
        mod("a", { constants: [{ name: "X", default: "1" }, { name: "Y" }] }),
        mod("b", { constants: [{ name: "Z", type: "int" }] }),
      ]);
      expect(constants.map((c) => c.id)).toEqual(["a.X", "a.Y", "b.Z"]);
      expect(constants[0]).toMatchObject({ kind: "constant", module: "a", default: "1" });
      expect(constants[2]).toMatchObject({ path: "b.Z", type: "int", repeated: false });
    });

    it("has an empty list for a module with no constants", () => {
      expect(docEntries([mod("a")]).constants).toEqual([]);
    });

    it("does not take an id from a card with the same path", () => {
      const e = docEntries([mod("a", { functions: [sym("X")], constants: [{ name: "X" }] })]);
      expect(e.functions[0]!.id).toBe("a.X");
      expect(e.constants[0]!.id).toBe("a.X~2");
      expect(e.constants[0]!.path).toBe("a.X");
    });

    it("marks a name that repeats across modules and kinds", () => {
      const e = docEntries([
        mod("a", { constants: [{ name: "X" }] }),
        mod("b", { constants: [{ name: "X" }], functions: [sym("f"), sym("g")] }),
        mod("c", { functions: [sym("g")], constants: [{ name: "f" }] }),
      ]);
      expect(e.constants.map((c) => c.repeated)).toEqual([true, true, true]);
      expect(e.functions.map((f) => f.repeated)).toEqual([true, true, true]);
    });

    it("sends an alias of a constant to its row", () => {
      const e = docEntries([
        mod("a", {
          aliases: [
            { name: "X", target: "a.core.X" },
            { name: "u", target: "a.core.u" },
          ],
        }),
        mod("a.core", { constants: [{ name: "X" }] }),
      ]);
      expect(e.aliases).toEqual([{ name: "X", module: "a", path: "a.X", targetId: "a.core.X" }]);
    });
  });
});
