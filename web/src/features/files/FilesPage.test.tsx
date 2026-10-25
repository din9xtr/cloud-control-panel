import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { entries, link, settings } from "@/test/fixtures";
import { renderWithApp } from "@/test/render";
import { server } from "@/test/server";
import { FilesPage } from "./FilesPage";

describe("FilesPage", () => {
  beforeEach(() => {
    server.use(
      http.get("/api/v1/files", ({ request }) => {
        const path = new URL(request.url).searchParams.get("path");
        return HttpResponse.json({ path, entries: path === "garden" ? entries : [] });
      }),
      http.get("/api/v1/links", () => HttpResponse.json({ items: [] })),
      http.get("/api/v1/settings", () => HttpResponse.json(settings)),
    );
  });

  it("lists folders first with sizes", async () => {
    renderWithApp(<FilesPage path="garden" />);
    const table = await screen.findByRole("table", { name: "Files in garden" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByRole("link", { name: "seedlings" })).toHaveAttribute("href", "/files/garden/seedlings");
    expect(within(rows[1]!).getAllByText("24.1 MB").length).toBeGreaterThan(0);
  });

  it("shows an empty state for empty folders", async () => {
    renderWithApp(<FilesPage path="empty" />);
    expect(await screen.findByText("This folder is empty")).toBeInTheDocument();
  });

  it("moves selected items to trash", async () => {
    let trashed: string[] = [];
    server.use(
      http.post("/api/v1/files/trash", async ({ request }) => {
        trashed = ((await request.json()) as { paths: string[] }).paths;
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderWithApp(<FilesPage path="garden" />);
    await userEvent.click((await screen.findAllByRole("checkbox", { name: "Select watering-schedule.md" }))[0]!);
    const toolbar = screen.getByRole("toolbar", { name: "Selection" });
    expect(toolbar).toHaveTextContent("1 selected");
    await userEvent.click(within(toolbar).getByRole("button", { name: "Move to Trash" }));
    await waitFor(() => expect(trashed).toEqual(["garden/watering-schedule.md"]));
    expect(await screen.findByRole("status")).toHaveTextContent("Moved watering-schedule.md to Trash");
  });

  it("creates a 24 hour link by default from the actions menu", async () => {
    let body: unknown;
    server.use(
      http.post("/api/v1/links", async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(link(), { status: 201 });
      }),
    );
    renderWithApp(<FilesPage path="garden" />);
    await userEvent.click(await screen.findByRole("button", { name: "Actions for watering-schedule.md" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Temporary link" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("radio", { name: "24 hours" })).toHaveAttribute("aria-checked", "true");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create link" }));
    expect(await within(dialog).findByLabelText("Link")).toHaveValue(`${window.location.origin}/s/q7Xv2mK9pLwR4tZaBcDeFg`);
    expect(body).toEqual({ path: "garden/watering-schedule.md", ttlSeconds: 86400, mode: "download" });
  });

  it("validates new folder names before sending", async () => {
    renderWithApp(<FilesPage path="garden" />);
    await screen.findByRole("table");
    await userEvent.click(screen.getByRole("button", { name: "New folder" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.type(within(dialog).getByLabelText("Name"), "a/b");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("Slashes are not allowed.");
  });

  it("creates upload-only links for folders from their actions", async () => {
    let body: unknown;
    server.use(http.post("/api/v1/links", async ({ request }) => {
      body = await request.json();
      return HttpResponse.json(link({ kind: "dir", mode: "upload", path: "garden/seedlings", name: "seedlings", url: "/share/q7Xv2mK9pLwR4tZaBcDeFg" }), { status: 201 });
    }));
    renderWithApp(<FilesPage path="garden" />);
    await userEvent.click(await screen.findByRole("button", { name: "Actions for seedlings" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Temporary link" }));
    const dialog = await screen.findByRole("dialog");
    await userEvent.selectOptions(within(dialog).getByLabelText("Link access"), "upload");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create link" }));
    expect(await within(dialog).findByLabelText("Link")).toHaveValue(`${window.location.origin}/share/q7Xv2mK9pLwR4tZaBcDeFg`);
    expect(body).toEqual({ path: "garden/seedlings", ttlSeconds: 86400, mode: "upload" });
  });
});
