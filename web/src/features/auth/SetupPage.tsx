import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { z } from "zod";
import { messageOf } from "@/api/client";
import { useSetup } from "@/api/queries";
import { type FieldErrors, fieldErrors, usernameSchema } from "@/features/settings/credentials";
import { Button } from "@/ui/Button";
import { Field } from "@/ui/Field";

const bytes = (value: string) => new TextEncoder().encode(value).length;

export const setupSchema = z
  .object({
    username: usernameSchema,
    password: z
      .string()
      .refine((v) => bytes(v) >= 8, "Use at least 8 characters.")
      .refine((v) => bytes(v) <= 72, "Use at most 72 bytes."),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ["confirm"], message: "Passwords do not match." });

export function SetupPage() {
  const setup = useSetup();
  const navigate = useNavigate();
  const [values, setValues] = useState({ username: "admin", password: "", confirm: "" });
  const [errors, setErrors] = useState<FieldErrors>({});
  const set = (key: keyof typeof values) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [key]: e.target.value }));

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const parsed = setupSchema.safeParse(values);
    if (!parsed.success) {
      setErrors(fieldErrors(parsed.error));
      return;
    }
    setErrors({});
    setup.mutate(
      { username: values.username, password: values.password },
      { onSuccess: () => void navigate({ to: "/files" }), onError: (e) => setErrors({ form: messageOf(e) }) },
    );
  };

  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-10 px-4 py-6">
      <div className="flex flex-col items-center gap-2.5 text-center">
        <samp className="font-mono text-[13px] text-fg-subtle">guest@localhost:~$ cloud init</samp>
        <h1 className="m-0 font-mono text-3xl font-medium tracking-[-0.01em]">cloud_</h1>
        <p className="m-0 max-w-[320px] text-[13px] text-fg-muted">Create the account you will use to sign in. You can change it later in Settings.</p>
      </div>
      <form onSubmit={submit} className="flex w-full max-w-[360px] flex-col gap-4" noValidate>
        <Field label="Username" mono autoComplete="username" autoCapitalize="none" spellCheck={false} value={values.username} onChange={set("username")} error={errors.username} />
        <Field label="Password" type="password" autoComplete="new-password" value={values.password} onChange={set("password")} error={errors.password} hint="At least 8 characters." autoFocus />
        <Field label="Repeat password" type="password" autoComplete="new-password" value={values.confirm} onChange={set("confirm")} error={errors.confirm} />
        {errors.form && (
          <p role="alert" className="m-0 text-[13px] text-danger">
            {errors.form}
          </p>
        )}
        <Button type="submit" variant="primary" size="lg" busy={setup.isPending}>
          Create account
        </Button>
      </form>
    </main>
  );
}
