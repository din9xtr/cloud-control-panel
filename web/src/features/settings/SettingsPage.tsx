import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type FormEvent, type ReactNode, useState } from "react";
import { messageOf, type Session, type Settings } from "@/api/client";
import { meQuery, sessionsQuery, settingsQuery, storageQuery, useChangeCredentials, useLogout, useRevokeSession, useUpdateSettings } from "@/api/queries";
import { PageBar, Page } from "@/layout/PageBar";
import { cn } from "@/lib/cn";
import { describeUserAgent, formatAgo, formatBytes } from "@/lib/format";
import { Button } from "@/ui/Button";
import { Dialog } from "@/ui/Dialog";
import { Badge, ErrorState, Loading, PageHeader } from "@/ui/Feedback";
import { control, Field, Select } from "@/ui/Field";
import { useToast } from "@/ui/Toast";
import { type FieldErrors, fieldErrors, passwordChangeSchema, quotaFromInput, quotaToInput, usernameChangeSchema } from "./credentials";

const day = 86400;
const retentionOptions = [7, 14, 30, 90, 180, 365].map((d) => ({ value: String(d), label: `${d} days` }));
const ttlOptions = [
  { value: String(3600), label: "1 hour" },
  { value: String(day), label: "24 hours" },
  { value: String(3 * day), label: "3 days" },
  { value: String(7 * day), label: "7 days" },
  { value: String(30 * day), label: "30 days" },
];

export function SettingsPage() {
  const settings = useQuery(settingsQuery);
  const jump = (id: string) => () => document.getElementById(`s-${id}`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  return (
    <>
      <PageBar
        prefix="05"
        title="Settings"
        actions={[
          { label: "Account", onSelect: jump("account") },
          { label: "Storage", onSelect: jump("storage") },
          { label: "Links", onSelect: jump("links") },
          { label: "Sessions", onSelect: jump("sessions") },
        ]}
      />
      <Page width="narrow">
        <PageHeader eyebrow="05 / settings.conf" title="Settings">
          Manage your account, storage limits, link defaults and signed-in devices.
        </PageHeader>
        <AccountSection />
        {settings.isPending && <Loading />}
        {settings.isError && <ErrorState message={messageOf(settings.error)} onRetry={() => void settings.refetch()} />}
        {settings.data && <PreferencesForm key={JSON.stringify(settings.data)} initial={settings.data} />}
        <SessionsSection />
      </Page>
    </>
  );
}

function Section({ name, children }: { name: string; children: ReactNode }) {
  return (
    <section aria-labelledby={`s-${name}`} className="flex animate-reveal scroll-mt-[76px] flex-col gap-4">
      <h2 id={`s-${name}`} className="type-section m-0 scroll-mt-[76px] capitalize">
        {name}
      </h2>
      <div className="overflow-hidden rounded-card border border-line bg-surface">{children}</div>
    </section>
  );
}

function Item({ title, hint, children, htmlFor }: { title: ReactNode; hint?: ReactNode; children?: ReactNode; htmlFor?: string }) {
  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-line px-6 py-5 last:border-b-0 max-sm:px-5">
      <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
        {htmlFor ? (
          <label htmlFor={htmlFor} className="type-body text-fg">
            {title}
          </label>
        ) : (
          <span className="type-body text-fg">{title}</span>
        )}
        {hint && <span className="type-small text-fg-subtle">{hint}</span>}
      </div>
      {children}
    </div>
  );
}

function AccountSection() {
  const { data: me } = useQuery(meQuery);
  const [editing, setEditing] = useState<"username" | "password" | null>(null);
  return (
    <Section name="account">
      <Item title="Username" hint={<span className="text-fg-muted">{me?.username}</span>}>
        <Button onClick={() => setEditing("username")}>Change username</Button>
      </Item>
      <Item title="Password" hint={me ? `Changed ${formatAgo(me.passwordChangedAt)}. Other sessions sign out after a change.` : undefined}>
        <Button onClick={() => setEditing("password")}>Change password</Button>
      </Item>
      {editing && <CredentialsDialog mode={editing} currentUsername={me?.username ?? ""} onClose={() => setEditing(null)} />}
    </Section>
  );
}

function CredentialsDialog({ mode, currentUsername, onClose }: { mode: "username" | "password"; currentUsername: string; onClose: () => void }) {
  const change = useChangeCredentials();
  const toast = useToast();
  const [values, setValues] = useState({ currentPassword: "", username: currentUsername, newPassword: "", confirmPassword: "" });
  const [errors, setErrors] = useState<FieldErrors>({});
  const set = (key: keyof typeof values) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [key]: e.target.value }));

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const parsed = mode === "username" ? usernameChangeSchema.safeParse(values) : passwordChangeSchema.safeParse(values);
    if (!parsed.success) {
      setErrors(fieldErrors(parsed.error));
      return;
    }
    setErrors({});
    const body = mode === "username" ? { currentPassword: values.currentPassword, username: values.username } : { currentPassword: values.currentPassword, newPassword: values.newPassword };
    change.mutate(body, {
      onSuccess: () => {
        toast.notify(mode === "username" ? "Username changed" : "Password changed. Other sessions were signed out.");
        onClose();
      },
      onError: (e) => setErrors({ form: messageOf(e) }),
    });
  };

  return (
    <Dialog
      open
      onClose={onClose}
      size="sm"
      eyebrow="Account"
      title={mode === "username" ? "Change username" : "Change password"}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" type="submit" form="credentials" busy={change.isPending}>
            Save
          </Button>
        </>
      }
    >
      <form id="credentials" onSubmit={submit} noValidate className="flex flex-col gap-4">
        <input type="text" name="username" autoComplete="username" value={currentUsername} readOnly hidden />
        {mode === "username" && <Field label="New username" mono autoComplete="off" autoCapitalize="none" spellCheck={false} value={values.username} onChange={set("username")} error={errors.username} autoFocus />}
        {mode === "password" && (
          <>
            <Field label="New password" type="password" autoComplete="new-password" value={values.newPassword} onChange={set("newPassword")} error={errors.newPassword} hint="At least 8 characters." autoFocus />
            <Field label="Repeat new password" type="password" autoComplete="new-password" value={values.confirmPassword} onChange={set("confirmPassword")} error={errors.confirmPassword} />
          </>
        )}
        <Field label="Current password" type="password" autoComplete="current-password" value={values.currentPassword} onChange={set("currentPassword")} error={errors.currentPassword} />
        {errors.form && (
          <p role="alert" className="type-small m-0 text-danger">
            {errors.form}
          </p>
        )}
      </form>
    </Dialog>
  );
}

function PreferencesForm({ initial }: { initial: Settings }) {
  const update = useUpdateSettings();
  const toast = useToast();
  const { data: storage } = useQuery(storageQuery);
  const [quota, setQuota] = useState(quotaToInput(initial.quotaBytes));
  const [retention, setRetention] = useState(String(initial.trashRetentionDays));
  const [ttl, setTtl] = useState(String(initial.defaultLinkTtlSeconds));
  const quotaBytes = quotaFromInput(quota);
  const dirty = quota !== quotaToInput(initial.quotaBytes) || retention !== String(initial.trashRetentionDays) || ttl !== String(initial.defaultLinkTtlSeconds);
  const retentionValues = retentionOptions.some((o) => o.value === retention) ? retentionOptions : [...retentionOptions, { value: retention, label: `${retention} days` }];
  const ttlValues = ttlOptions.some((o) => o.value === ttl) ? ttlOptions : [...ttlOptions, { value: ttl, label: `${Math.round(Number(ttl) / 3600)} hours` }];

  const discard = () => {
    setQuota(quotaToInput(initial.quotaBytes));
    setRetention(String(initial.trashRetentionDays));
    setTtl(String(initial.defaultLinkTtlSeconds));
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (quotaBytes === null) return;
    update.mutate(
      { quotaBytes, trashRetentionDays: Number(retention), defaultLinkTtlSeconds: Number(ttl) },
      { onSuccess: () => toast.notify("Settings saved"), onError: (e) => toast.fail(messageOf(e)) },
    );
  };

  return (
    <form onSubmit={submit} className="flex flex-col gap-7">
      <Section name="storage">
        <Item htmlFor="quota" title="Quota" hint={`Uploads stop when the limit is reached. Leave empty for no limit.${storage ? ` Disk has ${formatBytes(storage.disk.freeBytes)} free.` : ""}`}>
          <div className="flex items-center gap-2">
            <input
              id="quota"
              inputMode="decimal"
              placeholder="No limit"
              value={quota}
              onChange={(e) => setQuota(e.target.value)}
              aria-invalid={quotaBytes === null || undefined}
              className={cn(control, "type-small h-10 w-28 px-3.5 text-right tabular-nums max-sm:h-11 max-sm:text-[16px]")}
            />
            <span className="type-small text-fg-subtle">GB</span>
          </div>
        </Item>
        <Item htmlFor="retention" title="Keep deleted items" hint="Items in Trash are removed for good after this period.">
          <Select id="retention" label="Keep deleted items" value={retention} onChange={(e) => setRetention(e.target.value)} options={retentionValues} className="w-40" />
        </Item>
      </Section>
      <Section name="links">
        <Item htmlFor="ttl" title="Default link lifetime" hint="Preselected when you create a temporary link.">
          <Select id="ttl" label="Default link lifetime" value={ttl} onChange={(e) => setTtl(e.target.value)} options={ttlValues} className="w-40" />
        </Item>
      </Section>
      {dirty && (
        <div className="sticky bottom-[calc(env(safe-area-inset-bottom)+16px)] z-10 flex animate-rise items-center justify-end gap-2 rounded-card border border-line-strong bg-raised p-2 pl-4 shadow-float md:bottom-6">
          <span className="mr-auto font-mono text-xs text-fg-muted">unsaved changes</span>
          <Button onClick={discard}>Discard</Button>
          <Button variant="primary" type="submit" busy={update.isPending} disabled={quotaBytes === null}>
            Save changes
          </Button>
        </div>
      )}
    </form>
  );
}

function SessionsSection() {
  const { data, isPending } = useQuery(sessionsQuery);
  const logout = useLogout();
  const navigate = useNavigate();
  return (
    <Section name="sessions">
      {isPending && <Loading />}
      {data?.map((session) => (
        <SessionItem key={session.id} session={session} onSignOutHere={() => logout.mutate(undefined, { onSettled: () => void navigate({ to: "/login" }) })} />
      ))}
    </Section>
  );
}

function SessionItem({ session, onSignOutHere }: { session: Session; onSignOutHere: () => void }) {
  const revoke = useRevokeSession();
  const toast = useToast();
  return (
    <Item
      title={
        <>
          {describeUserAgent(session.userAgent)}
          {session.current && <span className="ml-2 align-middle"><Badge tone="info">This device</Badge></span>}
        </>
      }
      hint={
        <span className="tabular-nums">
          {session.ip || "unknown"} · {session.current ? "active now" : formatAgo(session.lastSeenAt)}
        </span>
      }
    >
      <Button
        variant="danger"
        busy={revoke.isPending}
        onClick={() => (session.current ? onSignOutHere() : revoke.mutate(session.id, { onSuccess: () => toast.notify("Session signed out"), onError: (e) => toast.fail(messageOf(e)) }))}
      >
        Sign out
      </Button>
    </Item>
  );
}
