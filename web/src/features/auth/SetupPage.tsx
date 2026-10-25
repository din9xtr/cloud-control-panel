import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { z } from "zod";
import { messageOf } from "@/api/client";
import { useSetup } from "@/api/queries";
import { type FieldErrors, fieldErrors, usernameSchema } from "@/features/settings/credentials";
import { Button } from "@/ui/Button";
import { Field } from "@/ui/Field";
import { AuthLayout } from "./AuthLayout";

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
    <AuthLayout title="Create your account" prompt="guest@localhost:~$ cloud init" intro="Create the account you will use to sign in. You can change it later in Settings.">
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <Field label="Username" mono autoComplete="username" autoCapitalize="none" spellCheck={false} value={values.username} onChange={set("username")} error={errors.username} />
        <Field label="Password" type="password" autoComplete="new-password" value={values.password} onChange={set("password")} error={errors.password} hint="At least 8 characters." autoFocus />
        <Field label="Repeat password" type="password" autoComplete="new-password" value={values.confirm} onChange={set("confirm")} error={errors.confirm} />
        {errors.form && (
          <p role="alert" className="type-small m-0 text-center text-danger">
            {errors.form}
          </p>
        )}
        <Button type="submit" variant="primary" size="lg" className="mt-2" busy={setup.isPending}>
          Create account
        </Button>
      </form>
    </AuthLayout>
  );
}
