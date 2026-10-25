export function safeRedirect(target: string | undefined, fallback = "/files"): string {
  if (!target || !target.startsWith("/") || target.startsWith("//") || target.includes("\\")) return fallback;
  try {
    const url = new URL(target, window.location.origin);
    if (url.origin !== window.location.origin) return fallback;
    if (url.pathname === "/login" || url.pathname === "/setup") return fallback;
    return url.pathname + url.search + url.hash;
  } catch {
    return fallback;
  }
}
