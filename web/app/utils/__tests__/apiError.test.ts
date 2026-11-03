import { describe, it, expect } from "vitest";
import { pageStatusForApiError, toPublicApiError } from "../apiError";

const INTERNAL = "http://api:8080/api/packages/nope";

function fetchError(statusCode: number | undefined) {
  return Object.assign(new Error(`[GET] "${INTERNAL}": ${statusCode ?? ""} failed`), {
    statusCode,
    request: INTERNAL,
    data: `upstream ${INTERNAL}`,
  });
}

describe("toPublicApiError", () => {
  it("keeps the status code", () => {
    expect(toPublicApiError(fetchError(404)).statusCode).toBe(404);
  });

  it("uses 503 when the API gave no response", () => {
    expect(toPublicApiError(fetchError(undefined)).statusCode).toBe(503);
  });

  it("removes the internal URL from every serialized field", () => {
    for (const status of [404, 500, undefined]) {
      const serialized = JSON.stringify(toPublicApiError(fetchError(status)));
      expect(serialized).not.toContain("api:8080");
    }
    const err = toPublicApiError(fetchError(502));
    expect(err.message).not.toContain("api:8080");
    expect(err.stack ?? "").not.toContain("api:8080");
    expect(JSON.stringify(err.cause) ?? "").not.toContain("api:8080");
  });
});

describe("pageStatusForApiError", () => {
  it("returns null without an error", () => {
    expect(pageStatusForApiError(null)).toBeNull();
    expect(pageStatusForApiError(undefined)).toBeNull();
  });

  it("maps a missing or invalid package to 404", () => {
    expect(pageStatusForApiError({ statusCode: 404 })).toBe(404);
    expect(pageStatusForApiError({ statusCode: 400 })).toBe(404);
  });

  it("maps every other failure to 503, so an outage never reads as a missing package", () => {
    for (const statusCode of [500, 502, 503, undefined]) {
      expect(pageStatusForApiError({ statusCode })).toBe(503);
    }
  });
});
