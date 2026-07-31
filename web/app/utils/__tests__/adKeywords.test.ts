import { describe, it, expect } from "vitest";
import { deriveAdKeywords } from "../adKeywords";

describe("deriveAdKeywords", () => {
  it("returns the most specific segment of Topic classifiers", () => {
    expect(deriveAdKeywords(["Topic :: Internet :: WWW/HTTP"])).toEqual(["www/http"]);
  });

  it("includes Framework classifiers", () => {
    expect(deriveAdKeywords(["Framework :: Django"])).toEqual(["django"]);
  });

  it("ignores non-subject classifier families", () => {
    expect(
      deriveAdKeywords([
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3.12",
        "Development Status :: 5 - Production/Stable",
      ]),
    ).toEqual([]);
  });

  it("deduplicates repeated segments", () => {
    expect(
      deriveAdKeywords(["Topic :: Software Development :: Testing", "Framework :: Testing"]),
    ).toEqual(["testing"]);
  });

  it("caps the result at five keywords", () => {
    const classifiers = ["a", "b", "c", "d", "e", "f", "g"].map((s) => `Topic :: ${s}`);
    expect(deriveAdKeywords(classifiers)).toHaveLength(5);
  });

  it("returns an empty array for null, undefined and empty input", () => {
    expect(deriveAdKeywords(null)).toEqual([]);
    expect(deriveAdKeywords(undefined)).toEqual([]);
    expect(deriveAdKeywords([])).toEqual([]);
  });

  it("ignores a bare family with no specific segment", () => {
    expect(deriveAdKeywords(["Topic"])).toEqual([]);
  });
});
