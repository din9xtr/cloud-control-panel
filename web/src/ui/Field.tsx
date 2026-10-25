import { ChevronDown, CircleAlert } from "lucide-react";
import { type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, useId } from "react";
import { cn } from "@/lib/cn";

export const control =
  "w-full rounded-control border border-line-strong bg-sunken text-fg outline-none transition-[border-color,box-shadow] duration-200 ease-swift placeholder:text-fg-subtle hover:border-fg-faint focus:border-accent focus:shadow-[0_0_0_3px_rgb(61_126_255/0.35)] aria-invalid:border-danger";

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
      <div className="relative">
        <input
          id={inputId}
          placeholder=" "
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn(control, "peer type-body h-[52px] px-3.5 pt-5 pb-1", mono && "font-mono")}
          {...rest}
        />
        <label
          htmlFor={inputId}
          className="type-micro pointer-events-none absolute top-[7px] left-3.5 text-fg-subtle transition-all duration-200 ease-swift peer-placeholder-shown:top-[14px] peer-placeholder-shown:text-[16px] peer-placeholder-shown:leading-[1.55] peer-focus:top-[7px] peer-focus:text-[12px] peer-focus:leading-[1.4] peer-focus:text-link peer-aria-invalid:text-danger"
        >
          {label}
        </label>
      </div>
      {error ? (
        <p id={`${inputId}-error`} role="alert" className="type-micro m-0 flex items-center gap-1.5 px-1 text-danger">
          <CircleAlert size={14} aria-hidden />
          {error}
        </p>
      ) : (
        hint && (
          <p id={`${inputId}-hint`} className="type-micro m-0 px-1 text-fg-subtle">
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
        <select id={selectId} className={cn(control, "type-small h-10 cursor-pointer appearance-none pr-9 pl-3.5 max-sm:h-11 max-sm:text-[16px]")} {...rest}>
          {options.map((o) => (
            <option key={o.value} value={o.value} className="bg-surface text-fg">
              {o.label}
            </option>
          ))}
        </select>
        <ChevronDown size={15} strokeWidth={2} aria-hidden className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-fg-subtle" />
      </div>
    </div>
  );
}

export function Meter({ value, label, className, tone = "default" }: { value: number; label: string; className?: string; tone?: "default" | "warning" | "danger" }) {
  const fill = { default: "bg-link", warning: "bg-warning", danger: "bg-danger" }[tone];
  return (
    <div role="meter" aria-label={label} aria-valuenow={value} aria-valuemin={0} aria-valuemax={100} className={cn("h-1.5 overflow-hidden rounded-sm bg-elevated", className)}>
      <div className={cn("h-full rounded-sm transition-[width] duration-700 ease-glide", fill)} style={{ width: `${value}%` }} />
    </div>
  );
}

export function usageTone(percentUsed: number): "default" | "warning" | "danger" {
  if (percentUsed >= 95) return "danger";
  if (percentUsed >= 85) return "warning";
  return "default";
}
