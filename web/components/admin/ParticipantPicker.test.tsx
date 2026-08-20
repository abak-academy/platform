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
let studentResponse = (path: string) => ({ data: students });

vi.mock("@/lib/api", () => ({
  authFetch: vi.fn(async (path: string) => {
    authFetchCalls.push(path);
    if (path.startsWith("/admin/schools")) {
      return { data: schools };
    }
    return studentResponse(path);
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
    value === "" ? (() => { throw new Error("SelectItem value cannot be empty"); })() :
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
    studentResponse = () => ({ data: students });
  });

  it("renders school-bearing and no-school rows without leaking null/undefined text", async () => {
    renderWithClient(<ParticipantPicker examId="exam-1" selected={[]} onChange={vi.fn()} />);

    expect(await screen.findByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText(/SMA Custom Tanpa Kode/)).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/\bnull\b/);
    expect(document.body.textContent).not.toMatch(/\bundefined\b/);
  });

  it("offers a no-school facet option and sends school_id=none when selected", async () => {
    renderWithClient(<ParticipantPicker examId="exam-1" selected={[]} onChange={vi.fn()} />);
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
      <ParticipantPicker examId="exam-1" selected={[]} onChange={onChange} />,
    );

    const row = await screen.findByRole("checkbox", { name: "Ada Lovelace" });
    expect(row).toHaveAttribute("aria-checked", "false");

    fireEvent.click(row);
    expect(onChange).toHaveBeenCalledWith(["s1"]);

    renderWithClient(<ParticipantPicker examId="exam-1" selected={["s1"]} onChange={onChange} />, client);

    const selectedRows = await screen.findAllByRole("checkbox", { name: "Ada Lovelace" });
    const selectedRow = selectedRows[selectedRows.length - 1];
    expect(selectedRow).toHaveAttribute("aria-checked", "true");
    expect(selectedRow.className).toMatch(/border-brand-600/);
  });

  it("loads and deduplicates 20-row pages while preserving selections across search", async () => {
    const third = { ...students[0], id: "s3", name: "Citra Dewi", username: "citra" };
    const fourth = { ...students[0], id: "s4", name: "Dewi Lestari", username: "dewi" };
    studentResponse = (path) => {
      if (path.includes("q=dewi")) return { data: [fourth] };
      if (path.includes("cursor=next-1")) return { data: [students[1], third] };
      return { data: students, next_cursor: "next-1" };
    };

    function Harness() {
      const [selected, setSelected] = React.useState<string[]>([]);
      return (
        <>
          <ParticipantPicker examId="exam-1" selected={selected} onChange={setSelected} />
          <output data-testid="selected">{selected.join(",")}</output>
        </>
      );
    }

    renderWithClient(<Harness />);
    fireEvent.click(await screen.findByRole("checkbox", { name: "Ada Lovelace" }));
    fireEvent.click(screen.getByText("Muat lebih banyak"));

    expect(await screen.findByText("Citra Dewi")).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox", { name: "Budi Santoso" })).toHaveLength(1);
    fireEvent.click(screen.getByText("Pilih semua"));
    expect(screen.getByTestId("selected")).toHaveTextContent("s1,s2,s3");

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "dewi" } });
    expect(await screen.findByText("Dewi Lestari", {}, { timeout: 1000 })).toBeInTheDocument();
    expect(screen.queryByText("Ada Lovelace")).not.toBeInTheDocument();
    expect(screen.getByTestId("selected")).toHaveTextContent("s1,s2,s3");
    expect(authFetchCalls).toContain(
      "/admin/exam-grants/students/search?q=dewi&exam_id=exam-1&limit=20",
    );
  });
});

describe("ParticipantPicker (school-scoped mode)", () => {
  beforeEach(() => {
    authFetchCalls.length = 0;
    studentResponse = (path) =>
      path.includes("cursor=next-1")
        ? { data: [{ ...students[0], id: "s3", name: "Citra Dewi" }] }
        : { data: students, next_cursor: "next-1" };
  });

  it("requests exam-scoped 20-row pages and follows next_cursor", async () => {
    renderWithClient(
      <ParticipantPicker examId="exam-1" schoolId="school-1" selected={[]} onChange={vi.fn()} />,
    );

    await screen.findByText("Ada Lovelace");
    expect(authFetchCalls).toContain(
      "/admin/students?limit=20&school_id=school-1&exam_id=exam-1",
    );
    fireEvent.click(screen.getByText("Muat lebih banyak"));
    expect(await screen.findByText("Citra Dewi")).toBeInTheDocument();
    expect(authFetchCalls).toContain(
      "/admin/students?cursor=next-1&limit=20&school_id=school-1&exam_id=exam-1",
    );
  });

  it("offers and clears canonical jenjang filters", async () => {
    studentResponse = () => ({
      data: students.map((student) => ({ ...student, jenjang: "" })),
    });
    renderWithClient(
      <ParticipantPicker examId="exam-1" schoolId="school-1" selected={[]} onChange={vi.fn()} />,
    );

    await screen.findByText("Ada Lovelace");
    const jenjang = screen.getAllByRole("combobox")[0];
    fireEvent.change(jenjang, { target: { value: "SMA" } });
    await waitFor(() =>
      expect(authFetchCalls.some((path) => path.includes("jenjang=SMA"))).toBe(true),
    );
    fireEvent.change(jenjang, { target: { value: "__all__" } });
    await waitFor(() =>
      expect(authFetchCalls[authFetchCalls.length - 1]).not.toContain("jenjang="),
    );
  });
});
