import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import SystemAuditPage from "./page";

const auditFilterCalls: Array<Record<string, unknown> | undefined> = [];

vi.mock("@/lib/hooks/admin-audit", () => ({
  useAdminAuditLog: (filters?: Record<string, unknown>) => {
    auditFilterCalls.push(filters);
    return { data: [], isLoading: false, error: null };
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
});
