const fallbacks = ["windows-1251", "koi8-r", "windows-1252"];

function hasBom(bytes: Uint8Array, ...bom: number[]): boolean {
  return bom.every((value, i) => bytes[i] === value);
}

function bomEncoding(bytes: Uint8Array): string | null {
  if (hasBom(bytes, 0xef, 0xbb, 0xbf)) return "utf-8";
  if (hasBom(bytes, 0xff, 0xfe)) return "utf-16le";
  if (hasBom(bytes, 0xfe, 0xff)) return "utf-16be";
  return null;
}

function cyrillicShare(text: string): number {
  const letters = text.match(/\p{L}/gu)?.length ?? 0;
  if (letters === 0) return 0;
  const cyrillic = text.match(/[А-яЁё]/g)?.length ?? 0;
  const lower = text.match(/[а-яё]/g)?.length ?? 0;
  return (cyrillic / letters) * (cyrillic > 0 ? lower / cyrillic : 0);
}

export function decodeText(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  const bom = bomEncoding(bytes);
  if (bom) return new TextDecoder(bom).decode(bytes);
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    const candidates = fallbacks.map((label) => new TextDecoder(label).decode(bytes));
    const [windows, koi] = candidates;
    if (windows !== undefined && koi !== undefined && Math.max(cyrillicShare(windows), cyrillicShare(koi)) > 0.3) {
      return cyrillicShare(windows) >= cyrillicShare(koi) ? windows : koi;
    }
    return candidates.at(-1) ?? "";
  }
}
