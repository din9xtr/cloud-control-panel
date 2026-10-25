import { z } from "zod";

const bytes = (value: string) => new TextEncoder().encode(value).length;

export const usernameSchema = z
  .string()
  .regex(/^[A-Za-z0-9._-]{3,32}$/, "Use 3-32 letters, digits, dots, dashes or underscores.");

export const passwordChangeSchema = z
  .object({
    currentPassword: z.string().min(1, "Enter your current password."),
    newPassword: z
      .string()
      .refine((v) => bytes(v) >= 8, "Use at least 8 characters.")
      .refine((v) => bytes(v) <= 72, "Use at most 72 bytes."),
    confirmPassword: z.string(),
  })
  .refine((v) => v.newPassword === v.confirmPassword, { path: ["confirmPassword"], message: "Passwords do not match." });

export const usernameChangeSchema = z.object({
  currentPassword: z.string().min(1, "Enter your current password."),
  username: usernameSchema,
});

export type FieldErrors = Partial<Record<string, string>>;

export function fieldErrors(error: z.ZodError): FieldErrors {
  const out: FieldErrors = {};
  for (const issue of error.issues) {
    const key = String(issue.path[0] ?? "form");
    out[key] ??= issue.message;
  }
  return out;
}

export const gib = 1024 ** 3;

export function quotaFromInput(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "") return 0;
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value < 0 || value > 1024 * 1024) return null;
  return Math.round(value * gib);
}

export function quotaToInput(bytes: number): string {
  if (bytes <= 0) return "";
  const value = bytes / gib;
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}
