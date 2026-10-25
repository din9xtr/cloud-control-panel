export type Crumb = { name: string; path: string };

export function splitPath(path: string): string[] {
  return path.split("/").filter(Boolean);
}

export function joinPath(dir: string, name: string): string {
  return dir ? `${dir}/${name}` : name;
}

export function parentPath(path: string): string {
  const parts = splitPath(path);
  parts.pop();
  return parts.join("/");
}

export function baseName(path: string): string {
  return splitPath(path).at(-1) ?? "";
}

export function crumbs(path: string): Crumb[] {
  const parts = splitPath(path);
  return parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }));
}

export function displayPath(path: string): string {
  return path ? `~/${path}` : "~";
}

export function filesHref(path: string): string {
  return path ? `/files/${splitPath(path).map(encodeURIComponent).join("/")}` : "/files";
}

export function contentUrl(path: string, inline = false): string {
  const query = new URLSearchParams({ path });
  if (inline) query.set("inline", "true");
  return `/api/v1/files/content?${query}`;
}

export function archiveUrl(paths: string[]): string {
  const query = new URLSearchParams();
  for (const p of paths) query.append("path", p);
  return `/api/v1/files/archive?${query}`;
}

export function validateName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) return "Name is required.";
  if (trimmed !== name) return "Remove spaces at the start or end.";
  if (name === "." || name === "..") return "This name is reserved.";
  if (/[/\\]/.test(name)) return "Slashes are not allowed.";
  if (new TextEncoder().encode(name).length > 255) return "Name is too long.";
  return null;
}
