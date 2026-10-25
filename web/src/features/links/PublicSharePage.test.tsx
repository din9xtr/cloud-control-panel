import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";
import { entries } from "@/test/fixtures";
import { renderWithApp } from "@/test/render";
import { server } from "@/test/server";
import { PublicSharePage } from "./PublicSharePage";

vi.mock("@/features/uploads/tusFactory", () => ({
  createTusFactory: () => (_file: File, _folder: string, callbacks: { onSuccess: () => void }) => ({ start: callbacks.onSuccess, pause: () => undefined, cancel: () => undefined }),
  tusFactory: () => ({ start: () => undefined, pause: () => undefined, cancel: () => undefined }),
}));

const token = "q7Xv2mK9pLwR4tZaBcDeFg";
const base = `/api/v1/public/links/${token}`;
const info = { name: "garden", mode: "download", expiresAt: "2026-11-01T12:00:00Z", maxUploadBytes: 5368709120 };

describe("PublicSharePage", () => {
  it("browses nested folders using paths relative to the shared folder", async () => {
    server.use(http.get(base, ({ request }) => {
      const path = new URL(request.url).searchParams.get("path") ?? "";
      return HttpResponse.json({ ...info, folder: { path, entries: path ? [] : entries.map((entry) => ({ ...entry, path: entry.name })) } });
    }));
    renderWithApp(<PublicSharePage token={token} />);
    await userEvent.click(await screen.findByRole("button", { name: "seedlings" }));
    expect(await screen.findByText("This folder is empty")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(await screen.findByRole("button", { name: "seedlings" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("checkbox", { name: "Select seed-catalog-2026.pdf" }));
    expect(screen.getByRole("button", { name: "Download selected (1)" })).toBeInTheDocument();
  });

  it("shows only an upload form for file requests and reports completion", async () => {
    server.use(http.get(base, () => HttpResponse.json({ ...info, mode: "upload" })));
    renderWithApp(<PublicSharePage token={token} />);
    const picker = await screen.findByLabelText("Choose files to upload");
    expect(screen.queryByRole("button", { name: "Download folder" })).not.toBeInTheDocument();
    expect(screen.getByText(/Its contents stay private/)).toBeInTheDocument();
    await userEvent.upload(picker, new File(["seeds"], "seed-catalog.txt", { type: "text/plain" }));
    expect(await screen.findByText("Uploaded")).toBeInTheDocument();
  });

  it("rejects oversized files before starting an upload", async () => {
    server.use(http.get(base, () => HttpResponse.json({ ...info, mode: "upload", maxUploadBytes: 2 })));
    renderWithApp(<PublicSharePage token={token} />);
    await userEvent.upload(await screen.findByLabelText("Choose files to upload"), new File(["seeds"], "seed-catalog.txt"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Each file must be");
    expect(screen.queryByText("Uploaded")).not.toBeInTheDocument();
  });

  it("explains revoked or expired links without requesting login", async () => {
    server.use(http.get(base, () => HttpResponse.json({ error: { code: "not_found", message: "link not found" } }, { status: 404 })));
    renderWithApp(<PublicSharePage token={token} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("This link is no longer available");
    expect(screen.queryByLabelText("Choose files to upload")).not.toBeInTheDocument();
  });
});
