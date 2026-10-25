import { useQuery } from "@tanstack/react-query";
import { Link, Outlet, useNavigate } from "@tanstack/react-router";
import { Folder, HardDrive, Link2, LogOut, SlidersHorizontal, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { linksQuery, meQuery, storageQuery, useLogout } from "@/api/queries";
import { UploadTray } from "@/features/uploads/UploadTray";
import { formatBytes, percent } from "@/lib/format";
import { IconButton } from "@/ui/Button";
import { Meter, usageTone } from "@/ui/Field";

type NavItem = { to: string; label: string; icon: ReactNode; mobile: boolean; badge?: number };

const iconProps = { size: 18, strokeWidth: 1.75, "aria-hidden": true } as const;

export function AppShell() {
  const { data: links } = useQuery(linksQuery);
  const nav: NavItem[] = [
    { to: "/files", label: "Files", icon: <Folder {...iconProps} />, mobile: true },
    { to: "/links", label: "Links", icon: <Link2 {...iconProps} />, mobile: true, badge: links?.length },
    { to: "/storage", label: "Storage", icon: <HardDrive {...iconProps} />, mobile: true },
    { to: "/trash", label: "Trash", icon: <Trash2 {...iconProps} />, mobile: false },
    { to: "/settings", label: "Settings", icon: <SlidersHorizontal {...iconProps} />, mobile: true },
  ];

  return (
    <div className="flex min-h-dvh">
      <a href="#main" className="sr-only z-50 rounded-md bg-fg px-3 py-2 text-canvas focus:not-sr-only focus:fixed focus:top-3 focus:left-3">
        Skip to content
      </a>
      <Sidebar nav={nav} />
      <main id="main" className="flex min-w-0 flex-1 flex-col pb-[calc(env(safe-area-inset-bottom)+64px)] md:pb-0">
        <Outlet />
      </main>
      <TabBar nav={nav.filter((n) => n.mobile)} />
      <UploadTray />
    </div>
  );
}

function Sidebar({ nav }: { nav: NavItem[] }) {
  const { data: me } = useQuery(meQuery);
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <aside className="sticky top-0 hidden h-dvh w-[248px] shrink-0 flex-col gap-7 border-r border-line px-3.5 py-5 md:flex">
      <div className="flex flex-col gap-1 px-2.5">
        <samp className="font-mono text-xs text-fg-subtle">{me?.username ?? "user"}@cloud:~$</samp>
        <span className="font-mono text-lg font-medium tracking-[-0.01em]">cloud_</span>
      </div>
      <nav aria-label="Primary" className="flex flex-col gap-0.5">
        {nav.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className="flex h-9 items-center gap-3 rounded-lg px-2.5 text-sm text-fg-muted no-underline transition-colors hover:bg-surface hover:text-fg"
            activeProps={{ className: "bg-surface !text-fg", "aria-current": "page" }}
          >
            {item.icon}
            {item.label}
            {item.badge ? <span className="ml-auto font-mono text-xs text-fg-subtle">{item.badge}</span> : null}
          </Link>
        ))}
      </nav>
      <div className="mt-auto flex flex-col gap-3">
        <StorageSummary />
        <div className="flex items-center justify-between border-t border-line px-2.5 pt-3">
          <span className="truncate font-mono text-xs text-fg-subtle">{me?.username}</span>
          <IconButton
            label="Sign out"
            size="sm"
            onClick={() => logout.mutate(undefined, { onSettled: () => void navigate({ to: "/login" }) })}
          >
            <LogOut size={15} aria-hidden />
          </IconButton>
        </div>
      </div>
    </aside>
  );
}

function StorageSummary() {
  const { data } = useQuery(storageQuery);
  if (!data) return null;
  const limit = data.quotaBytes > 0 ? data.quotaBytes : data.disk.totalBytes;
  const used = percent(data.usedBytes, limit);
  return (
    <Link to="/storage" className="flex flex-col gap-2.5 rounded-lg px-2.5 py-2 no-underline hover:bg-surface">
      <span className="flex items-baseline justify-between">
        <span className="label-caps">Storage</span>
        <span className="font-mono text-xs text-fg-muted">{used}%</span>
      </span>
      <Meter value={used} label="Storage used" tone={usageTone(used)} />
      <span className="font-mono text-xs text-fg-muted">
        {formatBytes(data.usedBytes)} of {formatBytes(limit)}
      </span>
    </Link>
  );
}

function TabBar({ nav }: { nav: NavItem[] }) {
  return (
    <nav aria-label="Primary" className="fixed inset-x-0 bottom-0 z-20 flex border-t border-line bg-canvas/86 px-2 pt-1.5 backdrop-blur-xl safe-bottom md:hidden">
      {nav.map((item) => (
        <Link
          key={item.to}
          to={item.to}
          className="flex min-h-[52px] flex-1 flex-col items-center justify-center gap-1 text-[11px] text-fg-subtle no-underline"
          activeProps={{ className: "!text-fg", "aria-current": "page" }}
        >
          {item.icon}
          {item.label}
        </Link>
      ))}
    </nav>
  );
}
