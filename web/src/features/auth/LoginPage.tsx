import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";
import { ApiError } from "@/api/client";
import { useLogin } from "@/api/queries";
import { Button } from "@/ui/Button";
import { safeRedirect } from "@/lib/redirect";
import { Field } from "@/ui/Field";
import { AuthLayout } from "./AuthLayout";

function loginError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "too_many_requests") return "Too many attempts. Wait a few minutes and try again.";
    if (error.code === "busy") return "The server is busy. Try again in a moment.";
    if (error.code === "invalid_credentials") return "Wrong username or password.";
  }
  return "Could not sign in. Check your connection.";
}

export function LoginPage({ redirect = "/files" }: { redirect?: string }) {
  const login = useLogin();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const submit = (event: FormEvent) => {
    event.preventDefault();
    login.mutate(
      { username: username.trim(), password },
      { onSuccess: () => void navigate({ to: safeRedirect(redirect) }) },
    );
  };

  return (
    <AuthLayout title="Sign in" prompt="guest@localhost:~$ ssh cloud" intro="Your files, on your own server." footnote="store. share. stay in control.">
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <Field
          label="Username"
          name="username"
          mono
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          required
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
        <Field
          label="Password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {login.isError && (
          <p role="alert" className="type-small m-0 text-center text-danger">
            {loginError(login.error)}
          </p>
        )}
        <Button type="submit" variant="primary" size="lg" className="mt-2" busy={login.isPending} disabled={!username.trim() || !password}>
          Sign in
        </Button>
      </form>
    </AuthLayout>
  );
}
