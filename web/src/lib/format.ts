const units = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  const digits = exponent === 0 || value >= 100 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[exponent]}`;
}

const sameYear = new Intl.DateTimeFormat("en", { month: "short", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
const otherYear = new Intl.DateTimeFormat("en", { year: "numeric", month: "short", day: "2-digit" });

export function formatDate(iso: string, now: Date = new Date()): string {
  const date = new Date(iso);
  return date.getFullYear() === now.getFullYear() ? sameYear.format(date) : otherYear.format(date);
}

export function formatRemaining(iso: string, now: Date = new Date()): string {
  const ms = new Date(iso).getTime() - now.getTime();
  if (ms <= 0) return "expired";
  const minutes = Math.floor(ms / 60_000);
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  if (days > 0) return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
  if (hours > 0) return `${hours}h ${String(minutes % 60).padStart(2, "0")}m`;
  return `${Math.max(minutes, 1)}m`;
}

export function formatAgo(iso: string, now: Date = new Date()): string {
  const seconds = Math.round((now.getTime() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.round(hours / 24);
  if (days < 45) return `${days} days ago`;
  const months = Math.round(days / 30);
  return months < 12 ? `${months} months ago` : `${Math.round(months / 12)} years ago`;
}

export function formatSpeed(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`;
}

export function percent(part: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, Math.round((part / total) * 100)));
}

const browsers: [RegExp, string][] = [
  [/Edg\//, "Edge"],
  [/Firefox\//, "Firefox"],
  [/Chrome\//, "Chrome"],
  [/Safari\//, "Safari"],
];

const systems: [RegExp, string][] = [
  [/iPhone|iPad/, "iOS"],
  [/Android/, "Android"],
  [/Mac OS X/, "macOS"],
  [/Windows/, "Windows"],
  [/Linux/, "Linux"],
];

export function describeUserAgent(ua: string): string {
  const browser = browsers.find(([re]) => re.test(ua))?.[1];
  const system = systems.find(([re]) => re.test(ua))?.[1];
  if (!browser && !system) return ua ? ua.slice(0, 40) : "Unknown device";
  return [browser, system].filter(Boolean).join(" · ");
}
