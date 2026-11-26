import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import SystemAuditPage from "./page";

let auditEntries: unknown[] = [];
const auditFilterCalls: Array<Record<string, unknown> | undefined> = [];

vi.mock("@/lib/hooks/admin-audit", () => ({
  useAdminAuditLog: (filters?: Record<string, unknown>) => {
    auditFilterCalls.push(filters);
    return { data: auditEntries, isLoading: false, error: null };
  },
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

describe("SystemAuditPage", () => {
  beforeEach(() => {
    auditFilterCalls.length = 0;
  });

  it("debounces the search box before it reaches the audit query", () => {
    vi.useFakeTimers();
    render(<SystemAuditPage />);

    fireEvent.change(screen.getByPlaceholderText("audit_search_placeholder"), {
      target: { value: "create" },
    });

    expect(auditFilterCalls.some((filters) => filters?.q !== undefined)).toBe(false);

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(auditFilterCalls.at(-1)?.q).toBe("create");

    vi.useRealTimers();
  });

  it("renders entries as a timeline, not a table", () => {
    auditEntries = [
      { id: "1", action: "user.create", actor_name: "Ani", actor_email: null, target_type: "user", target_id: "u1", created_at: "2026-11-01T00:00:00Z" },
      { id: "2", action: "user.delete", actor_name: null, actor_email: null, target_type: "user", target_id: "u2", created_at: "2026-11-02T00:00:00Z" },
    ];
    render(<SystemAuditPage />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.getByText("Ani")).toBeInTheDocument();
    expect(screen.getByText("System")).toBeInTheDocument();
    auditEntries = [];
  });
});
