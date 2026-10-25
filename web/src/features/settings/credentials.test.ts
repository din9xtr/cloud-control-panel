import { describe, expect, it } from "vitest";
import { fieldErrors, gib, passwordChangeSchema, quotaFromInput, quotaToInput, usernameChangeSchema } from "./credentials";

describe("credential schemas", () => {
  it("rejects mismatched and short passwords", () => {
    const result = passwordChangeSchema.safeParse({ currentPassword: "old", newPassword: "short", confirmPassword: "other" });
    expect(result.success).toBe(false);
    if (!result.success) {
      expect(fieldErrors(result.error)).toMatchObject({ newPassword: "Use at least 8 characters.", confirmPassword: "Passwords do not match." });
    }
  });

  it("accepts a valid username change", () => {
    expect(usernameChangeSchema.safeParse({ currentPassword: "x", username: "owner.1" }).success).toBe(true);
    expect(usernameChangeSchema.safeParse({ currentPassword: "x", username: "no spaces" }).success).toBe(false);
  });
});

describe("quota input", () => {
  it("maps empty to unlimited and gigabytes to bytes", () => {
    expect(quotaFromInput("")).toBe(0);
    expect(quotaFromInput("70")).toBe(70 * gib);
    expect(quotaFromInput("-1")).toBeNull();
    expect(quotaFromInput("abc")).toBeNull();
    expect(quotaToInput(70 * gib)).toBe("70");
    expect(quotaToInput(0)).toBe("");
  });
});
