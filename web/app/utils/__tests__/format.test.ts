import { describe, it, expect } from "vitest";
import { formatDate, formatDownloads, formatSize } from "../format";

describe("formatDate", () => {
  it("formats an ISO timestamp as a US medium date", () => {
    // Mid-day UTC so a local-TZ day shift can't flake the assertion.
    expect(formatDate("2024-05-21T12:00:00Z")).toMatch(/May 2[01], 2024/);
  });

  it("returns em-dash for empty input", () => {
    expect(formatDate("")).toBe("—");
  });
});

describe("formatDownloads", () => {
  it("formats sub-thousand values verbatim", () => {
    expect(formatDownloads(999)).toBe("999");
  });

  it("formats thousands with no decimal", () => {
    expect(formatDownloads(1500)).toBe("2K");
  });

  it("formats millions with one decimal", () => {
    expect(formatDownloads(2_500_000)).toBe("2.5M");
  });

  it("formats billions with one decimal", () => {
    expect(formatDownloads(3_200_000_000)).toBe("3.2B");
  });
});

describe("formatSize", () => {
  it("returns em-dash for zero", () => {
    expect(formatSize(0)).toBe("—");
  });

  it("formats bytes", () => {
    expect(formatSize(512)).toBe("512 B");
  });

  it("formats KB with no decimal", () => {
    expect(formatSize(150000)).toBe("146 KB");
  });

  it("formats MB with one decimal", () => {
    expect(formatSize(5_242_880)).toBe("5.0 MB");
  });
});
