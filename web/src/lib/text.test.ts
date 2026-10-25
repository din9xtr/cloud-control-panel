import { describe, expect, it } from "vitest";
import { decodeText } from "./text";

const sample = "Поливать томаты каждое утро. Ёжик и «тест».";

function bytes(...values: number[][]): ArrayBuffer {
  return new Uint8Array(values.flat()).buffer;
}

function windows1251(text: string): number[] {
  return [...text].map((char) => {
    const code = char.codePointAt(0) ?? 0;
    if (code >= 0x410 && code <= 0x44f) return code - 0x410 + 0xc0;
    if (char === "Ё") return 0xa8;
    if (char === "ё") return 0xb8;
    if (char === "«") return 0xab;
    if (char === "»") return 0xbb;
    return code;
  });
}

const koiLetters = "юабцдефгхийклмнопярстужвьызшэщчъ";

function koi8r(text: string): number[] {
  return [...text].map((char) => {
    const lower = char.toLowerCase();
    const index = koiLetters.indexOf(lower);
    if (index >= 0) return (lower === char ? 0xc0 : 0xe0) + index;
    if (char === "ё") return 0xa3;
    if (char === "Ё") return 0xb3;
    return char.codePointAt(0) ?? 0;
  });
}

describe("decodeText", () => {
  it.each([
    ["UTF-8", bytes([...new TextEncoder().encode(sample)])],
    ["UTF-8 with BOM", bytes([0xef, 0xbb, 0xbf], [...new TextEncoder().encode(sample)])],
    ["UTF-16 LE with BOM", bytes([0xff, 0xfe], [...sample].flatMap((c) => [c.charCodeAt(0) & 0xff, c.charCodeAt(0) >> 8]))],
    ["Windows-1251", bytes(windows1251(sample))],
  ])("reads %s text", (_, buffer) => {
    expect(decodeText(buffer)).toBe(sample);
  });

  it("reads KOI8-R text", () => {
    const text = "Список покупок: мука, дрожжи, лимоны";
    expect(decodeText(bytes(koi8r(text)))).toBe(text);
  });

  it("falls back to Windows-1252 for Latin text", () => {
    expect(decodeText(bytes([0x63, 0x61, 0x66, 0xe9]))).toBe("café");
  });
});
