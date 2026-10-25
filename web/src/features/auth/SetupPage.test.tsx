import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";
import { renderWithApp } from "@/test/render";
import { server } from "@/test/server";
import { SetupPage } from "./SetupPage";

describe("SetupPage", () => {
  it("validates before creating the account", async () => {
    renderWithApp(<SetupPage />);
    await userEvent.type(await screen.findByLabelText("Password"), "short");
    await userEvent.type(screen.getByLabelText("Repeat password"), "other");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByText("Use at least 8 characters.")).toBeInTheDocument();
    expect(screen.getByText("Passwords do not match.")).toBeInTheDocument();
  });

  it("creates the first account and opens files", async () => {
    let body: unknown;
    server.use(
      http.post("/api/v1/setup", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({ user: { id: 1, username: "owner", passwordChangedAt: "2026-10-25T00:00:00Z" } }, { status: 201 });
      }),
    );
    const { router } = renderWithApp(<SetupPage />);
    const username = await screen.findByLabelText("Username");
    await userEvent.clear(username);
    await userEvent.type(username, "owner");
    await userEvent.type(screen.getByLabelText("Password"), "first-run-pass");
    await userEvent.type(screen.getByLabelText("Repeat password"), "first-run-pass");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/files"));
    expect(body).toEqual({ username: "owner", password: "first-run-pass" });
  });
});
