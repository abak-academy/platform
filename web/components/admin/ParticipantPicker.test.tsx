import * as React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ParticipantPicker } from "./ParticipantPicker";
import type { CrossSchoolStudent, School } from "@/lib/types";

const students: CrossSchoolStudent[] = [
  {
    id: "s1",
    name: "Ada Lovelace",
    username: "ada",
    jenjang: "SMA",
    status: "active",
    grade: 11,
    school_id: "sch-1",
    school_name: "SMA Negeri 1",
    unlisted_school_name: null,
    created_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "s2",
    name: "Budi Santoso",
    username: "budi",
    jenjang: "SMA",
    status: "active",
    grade: 11,
    school_id: null,
    school_name: null,
    unlisted_school_name: "SMA Custom Tanpa Kode",
    created_at: "2026-01-01T00:00:00Z",
  },
];

const schools: School[] = [{ id: "sch-1", name: "SMA Negeri 1" }];

const authFetchCalls: string[] = [];

vi.mock("@/lib/api", () => ({
  authFetch: vi.fn(async (path: string) => {
    authFetchCalls.push(path);
    if (path.startsWith("/admin/schools")) {
      return { data: schools };
    }
    return { data: students };
  }),
}));

vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    value,
    onValueChange,
    ...rest
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => (
    <select value={value} onChange={(event) => onValueChange(event.target.value)} {...rest}>
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => (
    <option value={value}>{children}</option>
  ),
}));

function renderWithClient(ui: React.ReactNode, qc?: QueryClient) {
  const client = qc ?? new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return { client, ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>) };
}

describe("ParticipantPicker (cross-school mode)", () => {
  beforeEach(() => {
    authFetchCalls.length = 0;
  });

  it("renders school-bearing and no-school rows without leaking null/undefined text", async () => {
    renderWithClient(<ParticipantPicker selected={[]} onChange={vi.fn()} />);

    expect(await screen.findByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText(/SMA Custom Tanpa Kode/)).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/\bnull\b/);
    expect(document.body.textContent).not.toMatch(/\bundefined\b/);
  });

  it("offers a no-school facet option and sends school_id=none when selected", async () => {
    renderWithClient(<ParticipantPicker selected={[]} onChange={vi.fn()} />);
    await screen.findByText("Ada Lovelace");

    expect(screen.getByText("Tanpa sekolah terdaftar")).toBeInTheDocument();

    // Selects render, in order: jenjang filter, grade filter, school facet.
    const comboboxes = screen.getAllByRole("combobox");
    const schoolFacet = comboboxes[comboboxes.length - 1];
    fireEvent.change(schoolFacet, { target: { value: "none" } });

    await waitFor(() =>
      expect(authFetchCalls.some((p) => p.includes("school_id=none"))).toBe(true),
    );
  });

  it("flips aria-checked and adds a visible selected state when a row is clicked", async () => {
    const onChange = vi.fn();
    const { client } = renderWithClient(
      <ParticipantPicker selected={[]} onChange={onChange} />,
    );

    const row = await screen.findByRole("checkbox", { name: "Ada Lovelace" });
    expect(row).toHaveAttribute("aria-checked", "false");

    fireEvent.click(row);
    expect(onChange).toHaveBeenCalledWith(["s1"]);

    renderWithClient(<ParticipantPicker selected={["s1"]} onChange={onChange} />, client);

    const selectedRows = await screen.findAllByRole("checkbox", { name: "Ada Lovelace" });
    const selectedRow = selectedRows[selectedRows.length - 1];
    expect(selectedRow).toHaveAttribute("aria-checked", "true");
    expect(selectedRow.className).toMatch(/border-brand-600/);
  });
});
