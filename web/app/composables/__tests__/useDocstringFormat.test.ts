import { describe, it, expect } from "vitest";
import { formatDocstring, renderDocstring } from "../useDocstringFormat";

describe("formatDocstring", () => {
  describe("docstrings whose first line has no indent", () => {
    const flaskStyle = [
      "Sometimes it is necessary to set additional headers in a view.",
      "    Views do not have to return response objects.",
      "",
      "    If view looked like this and you want to add a new header::",
      "",
      "        def index():",
      "            return render_template('index.html', foo=42)",
      "",
      "    You can now do something like this",
      "    without a full stop at the end",
    ].join("\n");

    it("renders the indented body as prose", () => {
      const result = formatDocstring(flaskStyle);
      expect(result).toContain("<p>If view looked like this and you want to add a new header</p>");
      expect(result).toContain("<p>You can now do something like this without a full stop");
    });

    it("keeps real code as a code block, with its own indent removed", () => {
      const result = formatDocstring(flaskStyle);
      expect((result.match(/<pre>/g) ?? []).length).toBe(1);
      expect(result).toContain("def index():\n    return render_template");
    });

    it("keeps a deeper block as code without a :: marker", () => {
      const result = formatDocstring("Summary.\n\n    Body text\n\n        x = compute()");
      expect(result).toContain("<p>Body text</p>");
      expect(result).toContain("<pre><code");
      expect(result).toContain("x = compute()");
    });

    it("keeps doctest blocks as code", () => {
      const result = formatDocstring("Summary.\n\n    >>> add(1, 2)\n    3");
      expect(result).toContain(">>> add(1, 2)".replace(">>>", "&gt;&gt;&gt;"));
      expect(result).toContain("<pre><code");
    });
  });

  it("returns empty string for empty input", () => {
    expect(formatDocstring("")).toBe("");
    expect(formatDocstring("   ")).toBe("");
  });

  it("wraps single line in paragraph", () => {
    const result = formatDocstring("Hello world.");
    expect(result).toBe("<p>Hello world.</p>");
  });

  it("splits on blank lines into paragraphs", () => {
    const result = formatDocstring("First paragraph.\n\nSecond paragraph.");
    expect(result).toContain("<p>First paragraph.</p>");
    expect(result).toContain("<p>Second paragraph.</p>");
  });

  it("converts double backticks to code spans", () => {
    const result = formatDocstring("Use ``foo`` to do things.");
    expect(result).toContain("<code>foo</code>");
  });

  it("converts single backticks to code spans", () => {
    const result = formatDocstring("Use `bar` for that.");
    expect(result).toContain("<code>bar</code>");
  });

  it("strips rst role prefixes", () => {
    const result = formatDocstring("See :func:`print` and :class:`str`.");
    expect(result).toContain("<code>print</code>");
    expect(result).toContain("<code>str</code>");
    expect(result).not.toContain(":func:");
  });

  it("dedents indented docstrings", () => {
    const result = formatDocstring("    Summary line.\n\n    Extended description.");
    expect(result).toContain("<p>Summary line.</p>");
    expect(result).toContain("<p>Extended description.</p>");
  });

  it("detects indented code blocks", () => {
    const input = "Example::\n\n    foo()\n    bar()";
    const result = formatDocstring(input);
    expect(result).toContain('<pre><code class="language-python">');
    expect(result).toContain("foo()");
    expect(result).toContain("bar()");
  });

  it("detects >>> REPL blocks", () => {
    const input = "Example:\n\n    >>> print('hello')\n    hello";
    const result = formatDocstring(input);
    expect(result).toContain('<pre><code class="language-python">');
  });

  it("strips rst directives as muted notes", () => {
    const result = formatDocstring(".. versionchanged:: 2.0\n   The default changed.");
    expect(result).toContain("versionchanged");
    expect(result).not.toContain(".. version");
  });

  describe("NumPy section headings", () => {
    // Shape of numpy.add's docstring after the API removes structured sections.
    const numpyStyle = [
      "Add arguments element-wise.",
      "",
      "Notes",
      "-----",
      "Equivalent to `x1` + `x2` in terms of array broadcasting.",
      "",
      "Examples",
      "--------",
      ">>> np.add(1.0, 4.0)",
      "5.0",
      "",
      ">>> x1 = np.arange(9.0)",
      "",
      "See Also",
      "--------",
      "subtract : The opposite.",
    ].join("\n");

    it("renders the title as a heading without the dashes", () => {
      const result = formatDocstring(numpyStyle);
      expect(result).toContain(">Notes</h4>");
      expect(result).toContain(">Examples</h4>");
      expect(result).toContain(">See Also</h4>");
      expect(result).not.toContain("---");
    });

    it("keeps the example as code, without the title in the block", async () => {
      const seen: string[] = [];
      await renderDocstring(numpyStyle, async (code) => {
        seen.push(code);
        return "";
      });
      expect(seen).toEqual([">>> np.add(1.0, 4.0)\n5.0", ">>> x1 = np.arange(9.0)"]);
    });

    it("keeps text under a heading as a paragraph with inline markup", () => {
      const result = formatDocstring(numpyStyle);
      expect(result).toContain("<p>Equivalent to <code>x1</code> + <code>x2</code>");
    });

    it("handles a heading with a blank line before its body", () => {
      const result = formatDocstring("Examples\n--------\n\n>>> a");
      expect(result).toContain(">Examples</h4>");
      expect(result).toContain("<pre><code");
    });

    it("leaves dashes in doctest output alone", () => {
      const result = formatDocstring(">>> show()\nname\n----\nvalue");
      expect(result).not.toContain("<h4");
      expect(result).toContain("name\n----\nvalue");
    });

    it("leaves dashes in an indented block alone", () => {
      const result = formatDocstring("Table::\n\n    name\n    ----\n    value");
      expect(result).not.toContain("<h4");
      expect(result).toContain("name\n----\nvalue");
    });

    it("escapes the title", () => {
      const result = formatDocstring("<b>x</b>\n--------\ntext");
      expect(result).toContain("&lt;b&gt;x&lt;/b&gt;</h4>");
      expect(result).not.toContain("<b>");
    });

    it("needs at least three dashes", () => {
      expect(formatDocstring("Title\n--\ntext")).not.toContain("<h4");
    });
  });

  it("handles real click.echo docstring", () => {
    const input =
      "Print a message plus a newline to the given file or stdout. On\nfirst call, if ``err`` is false, a check is done whether the\nfile object has been set to non-blocking mode.\n\nExample::\n\n    click.echo('Hello World!')\n    click.echo(b'\\xe2\\x98\\x83', nl=False)";
    const result = formatDocstring(input);
    expect(result).toContain("<p>");
    expect(result).toContain("<code>err</code>");
    expect(result).toContain('<pre><code class="language-python">');
    expect(result).toContain("click.echo");
  });
});

describe("renderDocstring", () => {
  // Records the source each code block hands to the highlighter.
  function recorder(render: (code: string) => string = (code) => `<hl>${code.length}</hl>`) {
    const seen: string[] = [];
    const highlight = async (code: string) => {
      seen.push(code);
      return render(code);
    };
    return { seen, highlight };
  }

  it.each([
    ["an escaped entity", "s = '&amp;lt;'"],
    ["a double-escaped entity", "s = '&amp;amp;lt;'"],
    ["an entity that spells a tag", "s = '&lt;script&gt;'"],
    ["a quote entity", "s = '&amp;quot;'"],
    ["plain operators", "a & b < c > d"],
    ["quotes", `s = "x"`],
  ])("hands the highlighter the exact source for %s", async (_, code) => {
    const { seen, highlight } = recorder();
    await renderDocstring(`Example::\n\n    ${code}`, highlight);
    expect(seen).toEqual([code]);
  });

  it("does not reinterpret replacement patterns in highlighted code", async () => {
    const before = `Example::\n\n    a = "$&"\n\nNext.\n\n>>> b = "$'"\n\nEnd.`;
    const { highlight } = recorder((code) => `<hl>${code}</hl>`);
    const html = await renderDocstring(before, highlight);
    expect(html).toBe(
      `<p>Example</p>\n<hl>a = "$&"</hl>\n<p>Next.</p>\n<hl>>>> b = "$'"</hl>\n<p>End.</p>`,
    );
  });

  it("highlights every block when two blocks hold the same code", async () => {
    const { seen, highlight } = recorder((code) => `<hl>${code}</hl>`);
    const html = await renderDocstring(">>> a\n\nMiddle.\n\n>>> a", highlight);
    expect(seen).toEqual([">>> a", ">>> a"]);
    expect(html).toBe("<hl>>>> a</hl>\n<p>Middle.</p>\n<hl>>>> a</hl>");
  });

  it("keeps escaping prose around the code blocks", async () => {
    const { highlight } = recorder();
    const html = await renderDocstring("<img src=x onerror=alert(1)>\n\n>>> a", highlight);
    expect(html).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(html).not.toContain("<img");
  });

  it("escapes code in the plain fallback", () => {
    const html = formatDocstring(">>> '<script>alert(1)</script>' &amp;lt;");
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("&amp;amp;lt;");
    expect(html).not.toContain("<script>");
  });
});
