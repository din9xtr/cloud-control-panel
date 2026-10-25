import { ChevronsUpDown } from "lucide-react";
import { type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, useId } from "react";
import { cn } from "@/lib/cn";

const control =
  "h-10 w-full rounded-lg border border-line-strong bg-canvas px-3 text-[15px] text-fg placeholder:text-fg-subtle transition-colors outline-none focus:border-fg-muted aria-invalid:border-danger sm:h-9 sm:text-sm";

type FieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  hint?: ReactNode;
  error?: string | null;
  mono?: boolean;
};

export function Field({ label, hint, error, mono, className, id, ...rest }: FieldProps) {
  const generated = useId();
  const inputId = id ?? generated;
  const describedBy = error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined;
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <label htmlFor={inputId} className="text-[13px] text-fg-muted">
        {label}
      </label>
      <input id={inputId} aria-invalid={error ? true : undefined} aria-describedby={describedBy} className={cn(control, mono && "font-mono")} {...rest} />
      {error ? (
        <p id={`${inputId}-error`} role="alert" className="m-0 text-[13px] text-danger">
          {error}
        </p>
      ) : (
        hint && (
          <p id={`${inputId}-hint`} className="m-0 text-[13px] text-fg-subtle">
            {hint}
          </p>
        )
      )}
    </div>
  );
}

type SelectProps = SelectHTMLAttributes<HTMLSelectElement> & { label: string; options: { value: string; label: string }[] };

export function Select({ label, options, className, id, ...rest }: SelectProps) {
  const generated = useId();
  const selectId = id ?? generated;
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <label htmlFor={selectId} className="sr-only">
        {label}
      </label>
      <div className="relative">
        <select id={selectId} className={cn(control, "cursor-pointer appearance-none pr-9 hover:border-[#3d444d]")} {...rest}>
          {options.map((o) => (
            <option key={o.value} value={o.value} className="bg-surface text-fg">
              {o.label}
            </option>
          ))}
        </select>
        <ChevronsUpDown size={14} strokeWidth={1.75} aria-hidden className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-fg-subtle" />
      </div>
    </div>
  );
}

export function Meter({ value, label, className, tone = "default" }: { value: number; label: string; className?: string; tone?: "default" | "warning" | "danger" }) {
  const fill = { default: "bg-fg-muted", warning: "bg-warning", danger: "bg-danger" }[tone];
  return (
    <div role="meter" aria-label={label} aria-valuenow={value} aria-valuemin={0} aria-valuemax={100} className={cn("h-1 overflow-hidden rounded-full bg-line", className)}>
      <div className={cn("h-full rounded-full transition-[width] duration-500", fill)} style={{ width: `${value}%` }} />
    </div>
  );
}

export function usageTone(percentUsed: number): "default" | "warning" | "danger" {
  if (percentUsed >= 95) return "danger";
  if (percentUsed >= 85) return "warning";
  return "default";
}
