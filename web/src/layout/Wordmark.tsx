import { cn } from "@/lib/cn";

const sizes = { sm: "text-[13px]", md: "text-[16px]", lg: "text-[30px]" };

export function Wordmark({ size = "md", blink, className }: { size?: keyof typeof sizes; blink?: boolean; className?: string }) {
  return (
    <span className={cn("inline-flex items-baseline font-mono leading-none font-medium tracking-[-0.01em]", sizes[size], className)}>
      cloud
      <span aria-hidden className={cn("text-link", blink && "animate-caret")}>_</span>
    </span>
  );
}
