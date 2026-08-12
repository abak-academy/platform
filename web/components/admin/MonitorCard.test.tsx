import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Activity } from "lucide-react";
import { MonitorCard } from "./MonitorCard";

describe("MonitorCard", () => {
  it("renders the value and links to the page that owns it", () => {
    render(
      <MonitorCard testId="exam-sessions" href="/admin/exam/monitor" label="Sesi berjalan" value={3} icon={Activity} />,
    );
    expect(screen.getByTestId("exam-sessions")).toHaveTextContent("3");
    expect(screen.getByTestId("exam-sessions-link")).toHaveAttribute("href", "/admin/exam/monitor");
  });

  it("renders a zero muted rather than hiding the card", () => {
    render(
      <MonitorCard testId="exam-sessions" href="/admin/exam/monitor" label="Sesi berjalan" value={0} icon={Activity} />,
    );
    expect(screen.getByTestId("exam-sessions")).toHaveTextContent("0");
  });
});
