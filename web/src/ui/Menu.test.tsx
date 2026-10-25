import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Menu } from "./Menu";

afterEach(() => vi.unstubAllGlobals());

function setup(mobile = true) {
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: mobile, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  const select = vi.fn();
  const result = render(<><Menu label="File actions" title="garden" trigger="More" items={[{ label: "Rename", icon: null, onSelect: select }]} /><button>Outside</button></>);
  return { ...result, select };
}

describe("Menu", () => {
  it("dismisses the mobile sheet by tapping the backdrop without selecting an action", async () => {
    const { select, container } = setup();
    const trigger = screen.getByRole("button", { name: "File actions" });
    await userEvent.click(trigger);
    expect(container).not.toContainElement(screen.getByRole("menu"));
    expect(document.body).toHaveClass("overflow-hidden");
    await userEvent.click(screen.getByRole("button", { name: "Dismiss menu" }));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(document.body).not.toHaveClass("overflow-hidden");
    expect(select).not.toHaveBeenCalled();
    expect(trigger).toHaveFocus();
  });

  it("provides an explicit close button and handles Escape", async () => {
    setup();
    const trigger = screen.getByRole("button", { name: "File actions" });
    await userEvent.click(trigger);
    await userEvent.click(screen.getByRole("button", { name: "Close menu" }));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    await userEvent.click(trigger);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("selects a portalled item once and closes the sheet", async () => {
    const { select } = setup();
    await userEvent.click(screen.getByRole("button", { name: "File actions" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Rename" }));
    expect(select).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("still dismisses the desktop dropdown on an outside pointer", async () => {
    setup(false);
    await userEvent.click(screen.getByRole("button", { name: "File actions" }));
    await userEvent.click(screen.getByRole("button", { name: "Outside" }));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});
