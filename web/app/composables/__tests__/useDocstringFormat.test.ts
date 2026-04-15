import { describe, it, expect } from "vitest";
import { formatDocstring } from "../useDocstringFormat";

describe("formatDocstring", () => {
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
