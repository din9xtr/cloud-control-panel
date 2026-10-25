import type { CSSProperties } from "react";

export function stagger(index: number, base = 60): CSSProperties {
  const delay = `${base + index * 30}ms`;
  return { animationDelay: delay, transitionDelay: delay };
}
