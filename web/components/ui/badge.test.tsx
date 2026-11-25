import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Badge } from "./badge";

describe("Badge", () => {
  it.each(["teal", "gold", "sky", "berry", "ember"] as const)(
    "%s tone pairs the -50 background with the -700 text",
    (tone) => {
      render(<Badge variant={tone}>x</Badge>);
      const c = screen.getByText("x").className;
      expect(c).toContain(`bg-${tone}-50`);
      expect(c).toContain(`text-${tone}-700`);
    },
  );

  it("danger has no -700 step: its text is danger-600", () => {
    render(<Badge variant="danger">x</Badge>);
    expect(screen.getByText("x").className).toContain("bg-danger-50 text-danger-600");
  });

  it("mute, brand and navy use their own pairs", () => {
    render(
      <>
        <Badge variant="mute">mute</Badge>
        <Badge variant="brand">brand</Badge>
        <Badge variant="navy">navy</Badge>
      </>,
    );
    expect(screen.getByText("mute").className).toContain("bg-mute-50 text-mute-700");
    expect(screen.getByText("brand").className).toContain("bg-brand-50 text-brand-ink");
    expect(screen.getByText("navy").className).toContain("bg-brand-100 text-brand-800");
  });

  it("is a 26px pill in label-xs", () => {
    render(<Badge variant="teal">x</Badge>);
    const c = screen.getByText("x").className;
    expect(c).toContain("h-[26px]");
    expect(c).toContain("rounded-[var(--radius-pill)]");
    expect(c).toContain("type-label-xs");
  });

  it("live is ember solid with a pulsing dot; count is a 20px ember pill", () => {
    render(
      <>
        <Badge variant="live">live</Badge>
        <Badge variant="count">3</Badge>
      </>,
    );
    expect(screen.getByText("live").className).toContain("bg-ember-600 text-on-ember");
    expect(screen.getByText("live").className).toContain("before:animate-live");
    expect(screen.getByText("3").className).toContain("h-5");
    expect(screen.getByText("3").className).toContain("bg-ember-600");
  });
});
