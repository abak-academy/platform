import * as React from "react";

// Radix Select cannot be driven in jsdom, so tests that choose a value swap it
// for a native <select> that keeps the trigger's aria-label as its name.
type SelectProps = {
  value?: string;
  onValueChange?: (value: string) => void;
  children?: React.ReactNode;
};

export function SelectTrigger(_props: { "aria-label"?: string; id?: string; className?: string }) {
  return null;
}
export function SelectValue(_props: { placeholder?: string }) {
  return null;
}
export function SelectContent({ children }: { children?: React.ReactNode }) {
  return <>{children}</>;
}
export function SelectItem({ value, children }: { value: string; children?: React.ReactNode }) {
  return <option value={value}>{children}</option>;
}

export function Select({ value, onValueChange, children }: SelectProps) {
  const nodes = React.Children.toArray(children) as React.ReactElement<{ "aria-label"?: string }>[];
  const trigger = nodes.find((n) => n.type === SelectTrigger);
  return (
    <select
      aria-label={trigger?.props["aria-label"]}
      value={value ?? ""}
      onChange={(e) => onValueChange?.(e.target.value)}
    >
      <option value="" />
      {children}
    </select>
  );
}
