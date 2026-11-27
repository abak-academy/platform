import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import SystemAuditPage from "./page";

let auditEntries: unknown[] = [];
let auditTotal = 0;
const auditFilterCalls: Array<Record<string, unknown> | undefined> = [];

vi.mock("@/lib/hooks/admin-audit", () => ({
  useAdminAuditLog: (filters?: Record<string, unknown>) => {
    auditFilterCalls.push(filters);
    return { data: { data: auditEntries, total: auditTotal || auditEntries.length, page: filters?.page ?? 1, per_page: 20 }, isLoading: false, error: null };
  },
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

describe("SystemAuditPage", () => {
  beforeEach(() => {
    auditFilterCalls.length = 0;
    auditTotal = 0;
  });

  it("requests the next page from the server when Next is clicked", () => {
    auditEntries = [
      { id: "1", action: "user.create", actor_name: "Ani", actor_email: null, target_type: "user", target_id: "u1", created_at: "2026-11-01T00:00:00Z" },
    ];
    auditTotal = 45;
    render(<SystemAuditPage />);
    expect(auditFilterCalls.at(-1)).toMatchObject({ page: 1, limit: 20 });

    fireEvent.click(screen.getByRole("button", { name: "pagination_next" }));

    expect(auditFilterCalls.at(-1)).toMatchObject({ page: 2, limit: 20 });
    expect(screen.getByRole("button", { name: "pagination_prev" })).toBeEnabled();
    auditEntries = [];
  });

  it("hides the pager when everything fits on one page", () => {
    auditEntries = [
      { id: "1", action: "user.create", actor_name: "Ani", actor_email: null, target_type: "user", target_id: "u1", created_at: "2026-11-01T00:00:00Z" },
    ];
    render(<SystemAuditPage />);
    expect(screen.queryByRole("button", { name: "pagination_next" })).not.toBeInTheDocument();
    auditEntries = [];
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

  it("locks the oldest sort", () => {
    auditEntries = [
      { id: "1", action: "user.create", actor_name: "Ani", actor_email: null, target_type: "user", target_id: "u1", created_at: "2026-11-01T00:00:00Z" },
    ];
    render(<SystemAuditPage />);
    expect(screen.getByText("audit_sort_oldest_locked")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("combobox", { name: "list_toolbar_sort" }));
    expect(screen.getByRole("option", { name: "audit_sort_oldest" })).toHaveAttribute("aria-disabled", "true");
    auditEntries = [];
  });

  it("colours timeline nodes per action", () => {
    auditEntries = [
      { id: "1", action: "user.delete", actor_name: "A", actor_email: null, target_type: "user", target_id: "u1", created_at: "2026-11-01T00:00:00Z" },
      { id: "2", action: "user.update", actor_name: "A", actor_email: null, target_type: "user", target_id: "u2", created_at: "2026-11-01T00:00:00Z" },
      { id: "3", action: "user.create", actor_name: "A", actor_email: null, target_type: "user", target_id: "u3", created_at: "2026-11-01T00:00:00Z" },
    ];
    render(<SystemAuditPage />);
    const node = (i: number) => screen.getAllByRole("listitem")[i].querySelector("span.z-10")!;
    expect(node(0)).toHaveClass("bg-danger-600");
    expect(node(1)).toHaveClass("bg-brand-600");
    expect(node(2)).toHaveClass("bg-teal-600");
    auditEntries = [];
  });
});
