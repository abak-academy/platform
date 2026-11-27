import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Button } from "./button";

const cls = (name: string) => screen.getByRole("button", { name }).className;

describe("Button", () => {
  it("default is the ember primary pill with the CTA shadow", () => {
    render(<Button>Go</Button>);
    expect(cls("Go")).toContain("bg-ember-600");
    expect(cls("Go")).toContain("text-on-ember");
    expect(cls("Go")).toContain("shadow-cta");
    expect(cls("Go")).toContain("rounded-[var(--radius-pill)]");
  });

  it("secondary is brand solid", () => {
    render(<Button variant="secondary">Go</Button>);
    expect(cls("Go")).toContain("bg-brand-600");
    expect(cls("Go")).toContain("text-on-brand");
    expect(cls("Go")).toContain("shadow-brand");
  });

  it("outline, ghost, destructive, light and line-light follow the DS", () => {
    render(
      <>
        <Button variant="outline">outline</Button>
        <Button variant="ghost">ghost</Button>
        <Button variant="destructive">destructive</Button>
        <Button variant="light">light</Button>
        <Button variant="line-light">line</Button>
      </>,
    );
    expect(cls("outline")).toContain("border-[1.5px]");
    expect(cls("outline")).toContain("hover:border-ink-900");
    expect(cls("ghost")).toContain("hover:bg-surface-sunken");
    expect(cls("destructive")).toContain("bg-danger-600");
    expect(cls("destructive")).toContain("text-on-danger");
    expect(cls("light")).toContain("bg-white");
    expect(cls("line")).toContain("border-white");
  });

  it("dark is the prototype .ab-btn.dark: brand fill with the brand shadow", () => {
    render(<Button variant="dark">dk</Button>);
    expect(cls("dk")).toContain("bg-brand-600");
    expect(cls("dk")).toContain("text-on-brand");
    expect(cls("dk")).not.toContain("text-white");
    expect(cls("dk")).toContain("shadow-[var(--shadow-brand)]");
  });

  it("sizes map to the control heights", () => {
    render(
      <>
        <Button size="sm">sm</Button>
        <Button>md</Button>
        <Button size="lg">lg</Button>
      </>,
    );
    expect(cls("sm")).toContain("h-[var(--spacing-control-sm)]");
    expect(cls("md")).toContain("h-[var(--spacing-control-md)]");
    expect(cls("lg")).toContain("h-[var(--spacing-control-lg)]");
  });

  it("keeps the focus ring on brand and the disabled opacity", () => {
    render(<Button>Go</Button>);
    expect(cls("Go")).toContain("focus-visible:outline-brand-600");
    expect(cls("Go")).toContain("disabled:opacity-45");
  });

  it("keeps 18px padding with an icon and a transparent border so outline and filled match", () => {
    render(<><Button>md</Button><Button size="sm">sm</Button><Button size="lg">lg</Button></>);
    expect(cls("md")).toContain("px-[18px]");
    expect(cls("md")).not.toContain("has-[>svg]");
    expect(cls("md")).toContain("border-transparent");
    expect(cls("sm")).toContain("px-[13px]");
    expect(cls("sm")).toContain("gap-1.5");
    expect(cls("lg")).toContain("px-6");
    expect(cls("lg")).toContain("text-[15px]");
  });
});
