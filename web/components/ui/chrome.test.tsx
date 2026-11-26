import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Search } from "lucide-react";
import { Tabs, TabsList, TabsTrigger, TabCount } from "./tabs";
import { Progress } from "./progress";
import { Select, SelectTrigger, SelectValue } from "./select";
import { statusTone } from "./badge";
import { EmptyState } from "./empty-state";

describe("tabs", () => {
  it("renders the list as a paper pill with a border and scrolls on overflow", () => {
    render(
      <Tabs defaultValue="a">
        <TabsList><TabsTrigger value="a">A <TabCount>3</TabCount></TabsTrigger></TabsList>
      </Tabs>
    );
    expect(screen.getByRole("tablist")).toHaveClass("bg-surface", "border", "border-line", "overflow-x-auto", "max-w-full");
    expect(screen.getByText("3")).toHaveClass("bg-ember-600", "text-[10.5px]");
  });
});

describe("progress", () => {
  it("uses a sunken track and a teal fill by default", () => {
    render(<Progress value={40} />);
    expect(screen.getByRole("progressbar")).toHaveClass("bg-surface-sunken", "h-2");
    expect(screen.getByRole("progressbar").firstElementChild).toHaveClass("bg-teal-600");
  });

  it("supports ember and hatch fills", () => {
    const { rerender } = render(<Progress value={40} variant="ember" />);
    expect(screen.getByRole("progressbar").firstElementChild).toHaveClass("bg-ember-600");
    rerender(<Progress value={90} variant="hatch" />);
    expect(screen.getByRole("progressbar").firstElementChild!.className).toContain("repeating-linear-gradient(135deg");
  });
});

describe("select trigger size", () => {
  it.each([
    ["default", "h-[var(--spacing-input)]"],
    ["sm", "h-[var(--spacing-control-sm)]"],
    ["pill", "h-10"],
  ] as const)("%s", (size, cls) => {
    render(<Select><SelectTrigger size={size} aria-label="s"><SelectValue /></SelectTrigger></Select>);
    expect(screen.getByRole("combobox")).toHaveClass(cls);
    if (size !== "default") expect(screen.getByRole("combobox")).toHaveClass("rounded-full");
  });
});

describe("statusTone", () => {
  it("maps statuses onto Badge tones and falls back to mute", () => {
    expect(statusTone("published")).toBe("teal");
    expect(statusTone("Draft")).toBe("mute");
    expect(statusTone("pending")).toBe("gold");
    expect(statusTone("expired")).toBe("danger");
    expect(statusTone("bronze")).toBe("ember");
    expect(statusTone("???")).toBe("mute");
  });
});

describe("EmptyState framed", () => {
  it("wraps itself in a Card when framed", () => {
    const { container } = render(<EmptyState framed icon={Search} title="Kosong" />);
    expect(container.firstElementChild).toHaveAttribute("data-slot", "card");
    expect(screen.getByRole("heading", { name: "Kosong" })).toBeInTheDocument();
  });

  it("is unframed by default", () => {
    const { container } = render(<EmptyState icon={Search} title="Kosong" />);
    expect(container.firstElementChild).not.toHaveAttribute("data-slot");
  });
});
