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

  it("uses the default gradient when none is given", () => {
    const { container } = render(
      <DashboardHero icon={Shield} badge="b" name="n" subtitle="s" />,
    );
    expect(container.firstChild).toHaveStyle({ color: "#FFFFFF" });
  });

  it("keeps its mb-8 margin when className is omitted", () => {
    const { container } = render(
      <DashboardHero icon={Shield} badge="b" name="n" subtitle="s" />,
    );
    expect(container.firstChild).toHaveClass("mb-8");
  });
});
