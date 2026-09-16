import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
  useSchoolById: (id: string) => ({
    data: id === "school-outside" ? { id, name: "SMAN Outside Current Page", code: "OUTSIDE" } : null,
  }),
  useSchoolSearch: () => ({ data: { data: [] }, isFetching: false }),
}));

function renderFilter(value = "") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <SchoolFilterPicker value={value} onChange={() => undefined} label="Sekolah" allLabel="Semua sekolah" />
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

  it("hydrates a selected school outside the current result page", () => {
    renderFilter("school-outside");

    fireEvent.click(screen.getByRole("combobox", { name: "Sekolah" }));

    expect(screen.getByRole("option", { name: "SMAN Outside Current Page" })).toBeInTheDocument();
    expect(screen.queryByText("school-outside")).toBeNull();
  });
});
