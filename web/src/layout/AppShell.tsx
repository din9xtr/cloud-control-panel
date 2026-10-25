import { useQuery } from "@tanstack/react-query";
import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import { LogOut } from "lucide-react";
import { useEffect, useEffectEvent, useRef, useState } from "react";
import { meQuery, storageQuery, useLogout } from "@/api/queries";
import { UploadTray } from "@/features/uploads/UploadTray";
import { cn } from "@/lib/cn";
import { formatBytes, percent } from "@/lib/format";
import { stagger } from "@/lib/motion";
import { Meter, usageTone } from "@/ui/Field";
import { Wordmark } from "./Wordmark";

type NavItem = { to: string; label: string };

const nav: NavItem[] = [
  { to: "/files", label: "Files" },
  { to: "/links", label: "Links" },
  { to: "/storage", label: "Storage" },
  { to: "/trash", label: "Trash" },
  { to: "/settings", label: "Settings" },
];

export function AppShell() {
  return (
    <div className="flex min-h-dvh flex-col">
      <a href="#main" className="sr-only z-[70] rounded-control bg-accent px-4 py-2 text-on-accent focus:not-sr-only focus:fixed focus:top-3 focus:left-3">
        Skip to content
      </a>
      <TopBar />
      <main id="main" className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </main>
      <Footer />
      <UploadTray />
    </div>
  );
}

function useSignOut() {
  const logout = useLogout();
  const navigate = useNavigate();
  return () => logout.mutate(undefined, { onSettled: () => void navigate({ to: "/login" }) });
}

function useEscape(active: boolean, close: () => void) {
  const onEscape = useEffectEvent(close);
  useEffect(() => {
    if (!active) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onEscape();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active]);
}

function useScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const locked = document.body.classList.contains("overflow-hidden");
    document.body.classList.add("overflow-hidden");
    return () => {
      if (!locked) document.body.classList.remove("overflow-hidden");
    };
  }, [active]);
}

function TopBar() {
  const [account, setAccount] = useState(false);
  const [menu, setMenu] = useState(false);
  useEscape(account || menu, () => {
    setAccount(false);
    setMenu(false);
  });
  useScrollLock(menu);

  return (
    <header className="glass sticky top-0 z-40 border-b border-line">
      <nav aria-label="Primary" className="mx-auto flex h-14 max-w-[1120px] items-center gap-8 px-6 max-md:h-[52px] max-md:px-4">
        <Link to="/files" aria-label="Home" className="shrink-0 text-fg no-underline">
          <Wordmark />
        </Link>
        <ul className="m-0 hidden h-full list-none items-stretch gap-1 p-0 md:flex">
          {nav.map((item) => (
            <li key={item.to}>
              <Link
                to={item.to}
                className="type-small relative flex h-full items-center px-3 text-fg-subtle no-underline transition-colors duration-200 ease-swift after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:origin-left after:scale-x-0 after:rounded-full after:bg-accent after:transition-transform after:duration-280 after:ease-glide hover:text-fg"
                activeProps={{ className: "!text-fg after:!scale-x-100", "aria-current": "page" }}
              >
                {item.label}
              </Link>
            </li>
          ))}
        </ul>
        <span className="flex-1" />
        <AccountMenu open={account} onToggle={() => setAccount((v) => !v)} onClose={() => setAccount(false)} />
        <button
          type="button"
          aria-label="Open menu"
          aria-expanded={menu}
          aria-controls="global-menu"
          onClick={() => setMenu(true)}
          className="-mr-2 inline-flex size-11 shrink-0 cursor-pointer items-center justify-center text-fg-secondary md:hidden"
        >
          <MenuIcon open={false} />
        </button>
      </nav>
      <MobileMenu open={menu} onClose={() => setMenu(false)} />
    </header>
  );
}

function MenuIcon({ open }: { open: boolean }) {
  const bar = "absolute left-0.5 h-[1.75px] rounded-full bg-current transition-transform duration-280 ease-glide";
  return (
    <span aria-hidden className="relative block size-5">
      <span className={cn(bar, "top-[9px] w-4", open ? "rotate-45" : "-translate-y-1")} />
      <span className={cn(bar, "top-[9px]", open ? "w-4 -rotate-45" : "w-2.5 translate-y-1")} />
    </span>
  );
}

function AccountMenu({ open, onToggle, onClose }: { open: boolean; onToggle: () => void; onClose: () => void }) {
  const { data: me } = useQuery(meQuery);
  const signOut = useSignOut();
  const root = useRef<HTMLDivElement>(null);
  const name = me?.username ?? "user";
  const onOutside = useEffectEvent(onClose);

  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) onOutside();
    };
    document.addEventListener("pointerdown", onPointer);
    return () => document.removeEventListener("pointerdown", onPointer);
  }, [open]);

  const links = [
    { to: "/settings", label: "Settings", hint: "settings.conf" },
    { to: "/storage", label: "Storage", hint: "du -sh" },
    { to: "/trash", label: "Trash", hint: "~/.trash" },
  ];
  return (
    <div ref={root} className="relative hidden md:block">
      <button
        type="button"
        aria-label="Account"
        aria-expanded={open}
        aria-controls="account-menu"
        onClick={onToggle}
        className={cn(
          "inline-flex size-8 cursor-pointer items-center justify-center rounded-control border border-line-strong bg-raised font-mono text-[13px] text-fg-secondary uppercase transition-colors duration-200 ease-swift hover:border-fg-faint hover:text-fg",
          open && "border-accent text-fg",
        )}
      >
        {name.slice(0, 1)}
      </button>
      {open && (
        <div id="account-menu" className="absolute top-full right-0 mt-3 w-72 origin-top-right animate-drop rounded-card border border-line-strong bg-raised p-2 shadow-float">
          <div className="flex animate-enter flex-col gap-0.5 px-2.5 pt-2 pb-3" style={stagger(0)}>
            <samp className="font-mono text-xs text-fg-subtle">{name}@cloud:~$ whoami</samp>
            <span className="type-small font-semibold">{name}</span>
          </div>
          <div className="animate-enter border-y border-line px-2.5 py-3" style={stagger(1)}>
            <StorageSummary onNavigate={onClose} />
          </div>
          <ul className="m-0 flex list-none flex-col p-0 pt-1.5">
            {links.map((link, i) => (
              <li key={link.to} className="animate-enter" style={stagger(i + 2)}>
                <Link to={link.to} onClick={onClose} className="type-small flex h-9 items-center justify-between rounded-lg px-2.5 text-fg no-underline transition-colors hover:bg-elevated">
                  {link.label}
                  <span className="font-mono text-[11px] text-fg-faint">{link.hint}</span>
                </Link>
              </li>
            ))}
            <li className="animate-enter" style={stagger(links.length + 2)}>
              <button
                type="button"
                onClick={() => { onClose(); signOut(); }}
                className="type-small flex h-9 w-full cursor-pointer items-center gap-2 rounded-lg px-2.5 text-left text-danger transition-colors hover:bg-danger/10"
              >
                <LogOut size={14} aria-hidden />
                Sign out
              </button>
            </li>
          </ul>
        </div>
      )}
    </div>
  );
}

function StorageSummary({ onNavigate }: { onNavigate?: () => void }) {
  const { data } = useQuery(storageQuery);
  if (!data) return null;
  const limit = data.quotaBytes > 0 ? data.quotaBytes : data.disk.totalBytes;
  const used = percent(data.usedBytes, limit);
  return (
    <Link to="/storage" onClick={onNavigate} className="group flex flex-col gap-2 text-fg-secondary no-underline">
      <span className="flex items-baseline justify-between gap-3">
        <span className="type-micro group-hover:text-fg">{formatBytes(data.usedBytes)} of {formatBytes(limit)}</span>
        <span className="font-mono text-[11px] text-fg-subtle">{used}%</span>
      </span>
      <Meter value={used} label="Storage used" tone={usageTone(used)} />
    </Link>
  );
}

function MobileMenu({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { data: me } = useQuery(meQuery);
  const signOut = useSignOut();
  const item = cn("transition-[opacity,transform] duration-280 ease-glide", open ? "translate-x-0 opacity-100" : "-translate-x-2.5 opacity-0 !delay-0");
  const delay = (index: number) => stagger(index, 140);
  return (
    <div
      id="global-menu"
      role="dialog"
      aria-modal="true"
      aria-label="Menu"
      aria-hidden={!open}
      inert={!open}
      className={cn(
        "fixed inset-0 z-50 flex flex-col overflow-y-auto bg-sunken transition-[clip-path,visibility] duration-280 ease-swift md:hidden",
        open ? "visible [clip-path:inset(0_0_0_0)]" : "invisible [clip-path:inset(0_0_100%_0)]",
      )}
    >
      <div className="flex h-[52px] shrink-0 items-center justify-between border-b border-line px-4">
        <Link to="/files" aria-label="Home" onClick={onClose} className="text-fg no-underline">
          <Wordmark />
        </Link>
        <button type="button" aria-label="Close menu" onClick={onClose} className="-mr-2 inline-flex size-11 cursor-pointer items-center justify-center text-fg-secondary">
          <MenuIcon open={open} />
        </button>
      </div>
      <samp className={cn("px-6 pt-6 pb-2 font-mono text-xs text-fg-subtle", item)} style={delay(0)}>
        {me?.username ?? "user"}@cloud:~$ ls
      </samp>
      <ul className="m-0 flex list-none flex-col p-0 pb-6">
        {nav.map((link, i) => (
          <li key={link.to} className={item} style={delay(i + 1)}>
            <Link
              to={link.to}
              onClick={onClose}
              className="flex items-baseline gap-4 px-6 py-2 text-[26px] leading-tight font-semibold tracking-[-0.02em] text-fg-secondary no-underline"
              activeProps={{ className: "!text-fg", "aria-current": "page" }}
            >
              <span className="font-mono text-xs font-normal tracking-normal text-fg-faint">{String(i + 1).padStart(2, "0")}</span>
              {link.label}
            </Link>
          </li>
        ))}
      </ul>
      <div className={cn("mx-6 mt-auto flex flex-col gap-4 border-t border-line pt-5 pb-[max(env(safe-area-inset-bottom),24px)]", item)} style={delay(nav.length + 1)}>
        <StorageSummary onNavigate={onClose} />
        <button type="button" onClick={() => { onClose(); signOut(); }} className="type-body flex w-fit cursor-pointer items-center gap-2 p-0 text-danger">
          <LogOut size={16} aria-hidden />
          Sign out
        </button>
      </div>
    </div>
  );
}

function Footer() {
  return (
    <footer className="mt-20 border-t border-line">
      <div className="type-micro mx-auto flex max-w-[1120px] flex-wrap items-center gap-x-8 gap-y-3 px-6 py-6 text-fg-subtle max-md:px-4">
        <span className="flex items-center gap-3">
          <Wordmark size="sm" className="text-fg-secondary" />
          <span>self-hosted file storage</span>
        </span>
        <nav aria-label="Footer" className="flex flex-wrap gap-x-5 gap-y-2">
          {nav.map((item) => (
            <Link key={item.to} to={item.to} className="text-fg-subtle no-underline transition-colors hover:text-fg">
              {item.label}
            </Link>
          ))}
        </nav>
        <samp className="ml-auto font-mono max-sm:ml-0">store. share. stay in control.</samp>
      </div>
    </footer>
  );
}
