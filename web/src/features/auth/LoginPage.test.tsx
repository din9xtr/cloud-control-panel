import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { apiError } from "@/test/fixtures";
import { renderWithApp } from "@/test/render";
import { server } from "@/test/server";
import { LoginPage } from "./LoginPage";

const user = { id: 1, username: "admin", passwordChangedAt: "2026-01-01T00:00:00Z" };

describe("LoginPage", () => {
  it("keeps the submit button disabled until both fields are filled", async () => {
    renderWithApp(<LoginPage />);
    const submit = await screen.findByRole("button", { name: "Sign in" });
    expect(submit).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    expect(submit).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Password"), "secret");
    expect(submit).toBeEnabled();
  });

  it("shows a clear message for wrong credentials", async () => {
    server.use(http.post("/api/v1/auth/login", () => HttpResponse.json(apiError("invalid_credentials", "invalid username or password"), { status: 401 })));
    renderWithApp(<LoginPage />);
    await userEvent.type(await screen.findByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Password"), "wrong-pass");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Wrong username or password.");
  });

  it("explains throttling", async () => {
    server.use(http.post("/api/v1/auth/login", () => HttpResponse.json(apiError("too_many_requests", "too many attempts"), { status: 429 })));
    renderWithApp(<LoginPage />);
    await userEvent.type(await screen.findByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Password"), "wrong-pass");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Too many attempts");
  });

  it("navigates to files after signing in", async () => {
    server.use(http.post("/api/v1/auth/login", () => HttpResponse.json({ user })));
    const { router } = renderWithApp(<LoginPage />);
    await userEvent.type(await screen.findByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Password"), "right-pass");
    await userEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/files"));
  });
});
