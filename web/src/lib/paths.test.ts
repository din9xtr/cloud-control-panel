import { describe, expect, it } from "vitest";
import { archiveUrl, contentUrl, crumbs, filesHref, joinPath, parentPath, validateName } from "./paths";

describe("paths", () => {
  it("builds crumbs for nested folders", () => {
    expect(crumbs("projects/2026/q4")).toEqual([
      { name: "projects", path: "projects" },
      { name: "2026", path: "projects/2026" },
      { name: "q4", path: "projects/2026/q4" },
    ]);
    expect(crumbs("")).toEqual([]);
  });

  it("joins and splits", () => {
    expect(joinPath("", "a")).toBe("a");
    expect(joinPath("a", "b")).toBe("a/b");
    expect(parentPath("a/b/c")).toBe("a/b");
    expect(parentPath("a")).toBe("");
  });

  it("encodes urls", () => {
    expect(filesHref("отчёты/q 1")).toBe("/files/%D0%BE%D1%82%D1%87%D1%91%D1%82%D1%8B/q%201");
    expect(contentUrl("a b.pdf", true)).toBe("/api/v1/files/content?path=a+b.pdf&inline=true");
    expect(archiveUrl(["a", "b/c"])).toBe("/api/v1/files/archive?path=a&path=b%2Fc");
  });

  it.each([
    ["", "Name is required."],
    [" a", "Remove spaces at the start or end."],
    ["..", "This name is reserved."],
    ["a/b", "Slashes are not allowed."],
    ["report.pdf", null],
  ])("validates %j", (name, expected) => {
    expect(validateName(name)).toBe(expected);
  });
});
