import { describe, expect, it } from "vitest";
import { safeRedirect } from "./redirect";

describe("safeRedirect", () => {
  it.each([
    [undefined, "/files"],
    ["/files/garden?x=1", "/files/garden?x=1"],
    ["//evil.example", "/files"],
    ["/\\evil.example", "/files"],
    ["https://evil.example/files", "/files"],
    ["javascript:alert(1)", "/files"],
    ["/login", "/files"],
  ])("maps %j to %j", (input, expected) => {
    expect(safeRedirect(input)).toBe(expected);
  });
});
