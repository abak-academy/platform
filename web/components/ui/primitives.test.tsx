import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Card } from "./card";
import { Input } from "./input";
import { Select, SelectTrigger, SelectValue } from "./select";
import { Popover, PopoverContent } from "./popover";
import { Tabs, TabsList, TabsTrigger } from "./tabs";

describe("v2 primitive shells", () => {
  it("Card uses the card radius, line border and card shadow", () => {
    render(<Card data-testid="c" />);
    const c = screen.getByTestId("c").className;
    expect(c).toContain("rounded-[var(--radius-card)]");
    expect(c).toContain("border-line");
    expect(c).toContain("shadow-card");
  });

  it("Input is 46px with a 1.5px border and a brand focus halo", () => {
    render(<Input aria-label="n" />);
    const c = screen.getByLabelText("n").className;
    expect(c).toContain("h-[var(--spacing-input)]");
    expect(c).toContain("border-[1.5px]");
    expect(c).toContain("focus-visible:border-brand-600");
    expect(c).toContain("focus-visible:ring-brand-50");
  });

  it("Select trigger matches the Input geometry", () => {
    render(
      <Select>
        <SelectTrigger aria-label="s">
          <SelectValue />
        </SelectTrigger>
      </Select>,
    );
    const c = screen.getByLabelText("s").className;
    expect(c).toContain("h-[var(--spacing-input)]");
    expect(c).toContain("focus-visible:ring-brand-50");
  });

  it("Popover is capped at the popover width and sits on the popover layer", () => {
    render(
      <Popover open>
        <PopoverContent>body</PopoverContent>
      </Popover>,
    );
    const c = screen.getByText("body").className;
    expect(c).toContain("var(--container-popover)");
    expect(c).toContain("z-[var(--z-popover)]");
    expect(c).toContain("shadow-pop");
  });

  it("Tabs: pill track, brand-solid active trigger", () => {
    render(
      <Tabs defaultValue="a">
        <TabsList>
          <TabsTrigger value="a">A</TabsTrigger>
        </TabsList>
      </Tabs>,
    );
    expect(screen.getByRole("tablist").className).toContain("rounded-[var(--radius-pill)]");
    expect(screen.getByRole("tab", { name: "A" }).className).toContain("data-[state=active]:bg-brand-600");
  });
});
