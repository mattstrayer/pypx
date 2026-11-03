import { describe, it, expect } from "vitest";
import { highlightPython } from "../shikiHighlight";

describe("highlightPython outside SSR", () => {
  it("escapes the code it returns", async () => {
    const html = await highlightPython(`<script>alert("1")</script> &lt;`);
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
    expect(html).toContain("&amp;lt;");
  });
});
