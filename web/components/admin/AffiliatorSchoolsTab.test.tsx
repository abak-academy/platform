import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AffiliatorSchoolsTab } from "./AffiliatorSchoolsTab";

const useAffiliatorSchools = vi.fn();
const download = vi.fn();

vi.mock("@/lib/hooks/admin-affiliators", () => ({
  useAffiliatorSchools: (p: unknown) => useAffiliatorSchools(p),
  downloadAffiliatorSchoolsCsv: (p: unknown) => download(p),
  useAdminAffiliator: () => ({ data: undefined }),
  useAdminAffiliators: () => ({ data: { data: [] } }),
}));
vi.mock("@/lib/hooks/students", () => ({ useSchoolById: () => ({ data: undefined }) }));
vi.mock("@/components/SchoolFilterPicker", () => ({ SchoolFilterPicker: () => <div>school picker</div> }));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));

const row = {
  affiliator_id: "a1",
  affiliator_name: "Andi Affiliator",
  affiliator_email: "andi@example.test",
  affiliator_status: "active",
  school_id: "s1",
  school_name: "SMA Satu",
  school_code: "SMA1",
  school_npsn: "1234",
  school_status: "active",
  student_count: 7,
};

describe("AffiliatorSchoolsTab", () => {
  beforeEach(() => {
    useAffiliatorSchools.mockReset();
    download.mockReset();
    useAffiliatorSchools.mockReturnValue({ data: { data: [row], total: 1, limit: 20, offset: 0 }, isLoading: false, isError: false });
  });

  it("lists each school with its affiliator", () => {
    render(<AffiliatorSchoolsTab />);

    expect(screen.getByText("SMA Satu")).toBeInTheDocument();
    expect(screen.getByText("Andi Affiliator")).toBeInTheDocument();
    expect(screen.getByText("andi@example.test")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
  });

  it("passes the search text to the query", () => {
    render(<AffiliatorSchoolsTab />);

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "andi" } });

    expect(useAffiliatorSchools).toHaveBeenLastCalledWith(expect.objectContaining({ q: "andi", offset: 0 }));
  });

  it("exports the current search as CSV", () => {
    render(<AffiliatorSchoolsTab />);

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "andi" } });
    fireEvent.click(screen.getByRole("button", { name: /Ekspor CSV/i }));

    expect(download).toHaveBeenCalledWith(expect.objectContaining({ q: "andi" }));
  });

  it("shows the empty message", () => {
    useAffiliatorSchools.mockReturnValue({ data: { data: [], total: 0, limit: 20, offset: 0 }, isLoading: false, isError: false });
    render(<AffiliatorSchoolsTab />);

    expect(screen.getByText("Tidak ada sekolah yang ditugaskan ke affiliator.")).toBeInTheDocument();
  });
});
