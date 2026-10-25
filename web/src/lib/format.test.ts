import { describe, expect, it } from "vitest";
import { describeUserAgent, formatAgo, formatBytes, formatRemaining, percent } from "./format";

describe("formatBytes", () => {
  it.each([
    [0, "0 B"],
    [512, "512 B"],
    [1536, "1.5 KB"],
    [24.1 * 1024 ** 2, "24.1 MB"],
    [812 * 1024 ** 2, "812 MB"],
    [70 * 1024 ** 3, "70.0 GB"],
  ])("formats %d as %s", (bytes, expected) => {
    expect(formatBytes(bytes)).toBe(expected);
  });
});

describe("formatRemaining", () => {
  const now = new Date("2026-10-25T12:00:00Z");
  it.each([
    ["2026-10-26T12:00:00Z", "1d"],
    ["2026-10-26T11:04:00Z", "23h 04m"],
    ["2026-10-25T12:30:00Z", "30m"],
    ["2026-10-25T11:00:00Z", "expired"],
  ])("formats %s as %s", (iso, expected) => {
    expect(formatRemaining(iso, now)).toBe(expected);
  });
});

describe("formatAgo", () => {
  const now = new Date("2026-10-25T12:00:00Z");
  it("describes recent and old instants", () => {
    expect(formatAgo("2026-10-25T11:59:30Z", now)).toBe("just now");
    expect(formatAgo("2026-10-25T10:00:00Z", now)).toBe("2 h ago");
    expect(formatAgo("2026-01-25T12:00:00Z", now)).toBe("9 months ago");
  });
});

describe("percent", () => {
  it("clamps and handles zero totals", () => {
    expect(percent(5, 0)).toBe(0);
    expect(percent(28, 100)).toBe(28);
    expect(percent(200, 100)).toBe(100);
  });
});

describe("describeUserAgent", () => {
  it("names browser and system", () => {
    expect(describeUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15")).toBe("Safari · macOS");
    expect(describeUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Version/18.0 Mobile/15E148 Safari/604.1")).toBe("Safari · iOS");
    expect(describeUserAgent("")).toBe("Unknown device");
  });
});
