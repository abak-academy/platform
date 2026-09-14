import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SchoolFilterPicker } from "./SchoolFilterPicker";

const useAdminSchoolsCalls: unknown[] = [];

vi.mock("@/lib/hooks/admin-schools", () => ({
  useAdminSchools: (params: unknown) => {
    useAdminSchoolsCalls.push(params);
    return {
      data: { data: [{ id: "school-1", name: "SMAN 1 Jakarta" }] },
      isFetching: false,
    };
  },
}));

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: [{ id: "province-1", name: "DKI JAKARTA" }] }),
  useCitiesByProvince: () => ({ data: [{ id: "city-1", name: "KOTA JAKARTA PUSAT" }] }),
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchoolById: () => ({ data: null }),
  useSchoolSearch: () => ({ data: { data: [] }, isFetching: false }),
}));

function renderFilter() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <SchoolFilterPicker value="" onChange={() => undefined} label="Sekolah" allLabel="Semua sekolah" />
    </QueryClientProvider>,
  );
}

describe("SchoolFilterPicker", () => {
  beforeEach(() => {
    useAdminSchoolsCalls.length = 0;
  });

  it("stays compact and does not render the full SchoolPicker controls", () => {
    renderFilter();

    expect(screen.getByRole("combobox", { name: "Sekolah" })).toBeInTheDocument();
    expect(screen.queryByText("Provinsi")).not.toBeInTheDocument();
    expect(screen.queryByText("Kota/Kabupaten")).not.toBeInTheDocument();
    expect(screen.queryByText("Nama")).not.toBeInTheDocument();
    expect(screen.queryByText("NPSN")).not.toBeInTheDocument();
  });

  it("uses the bounded admin school list for page-level filtering", () => {
    renderFilter();

    expect(useAdminSchoolsCalls.at(-1)).toEqual({ q: undefined, status: "active", limit: 20 });
  });
});
