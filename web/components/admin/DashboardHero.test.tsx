import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Shield } from "lucide-react";
import { DashboardHero } from "./DashboardHero";

describe("DashboardHero", () => {
  it("renders the badge, name and subtitle", () => {
    render(
      <DashboardHero icon={Shield} badge="Admin Ujian" name="Budi" subtitle="Pantau sesi ujian." />,
    );
    expect(screen.getByText("Admin Ujian")).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Budi" })).toBeTruthy();
    expect(screen.getByText("Pantau sesi ujian.")).toBeTruthy();
  });

  it("renders as a plain page head with no gradient or margin", () => {
    const { container } = render(
      <DashboardHero icon={Shield} badge="b" name="n" subtitle="s" />,
    );
    const root = container.firstChild as HTMLElement;
    expect(root).not.toHaveClass("mb-8");
    expect(root.getAttribute("style")).toBeNull();
    expect(screen.getByRole("heading", { name: "n" })).toHaveClass("type-h1");
  });
});
