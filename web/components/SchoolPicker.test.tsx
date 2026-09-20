import * as React from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SchoolPicker } from "./SchoolPicker";
import type { SchoolOption } from "@/lib/types";

const searchCalls: Array<{ params: Record<string, unknown>; enabled: boolean }> = [];
let lastSearchKey = "";
const defaultSchools: SchoolOption[] = [
  {
    id: "school-1",
    name: "SMA Negeri 1 Jakarta",
    code: "SMAN1JKT",
    npsn: "12345678",
    school_types: ["SMA"],
    kota_name: "KOTA JAKARTA PUSAT",
    provinsi_name: "DKI JAKARTA",
  },
  {
    id: "school-2",
    name: "SMA Bina Bangsa",
    code: "SMABB",
    npsn: "87654321",
    school_types: ["SMA"],
    kota_name: "KOTA JAKARTA PUSAT",
    provinsi_name: "DKI JAKARTA",
  },
];
let schools = defaultSchools;
let hydratedSchool: SchoolOption | null = null;
let searchError = false;
let hasMore = false;
const fetchNextPage = vi.fn();

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: [{ id: "province-1", name: "DKI JAKARTA" }] }),
  useSchoolCities: () => ({ data: [{ id: "city-1", province_id: "province-1", name: "KOTA JAKARTA PUSAT" }] }),
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchoolById: () => ({ data: hydratedSchool }),
  useSchoolAutocomplete: (params: Record<string, unknown>, enabled: boolean) => {
    const key = JSON.stringify({ params, enabled });
    if (key !== lastSearchKey) {
      searchCalls.push({ params, enabled });
      lastSearchKey = key;
    }
    return { data: { pages: [{ data: enabled ? schools : [] }] }, isFetching: false, isError: searchError, hasNextPage: hasMore, fetchNextPage, refetch: vi.fn() };
  },
}));

vi.mock("@/components/ui/select", () => ({
  Select: ({ children, value, onValueChange, disabled }: { children: React.ReactNode; value: string; onValueChange: (value: string) => void; disabled?: boolean }) => (
    <select value={value} onChange={(event) => onValueChange(event.target.value)} disabled={disabled}>
      {children}
    </select>
  ),
  SelectTrigger: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectValue: ({ placeholder }: { placeholder?: string }) => <option value="__placeholder__">{placeholder}</option>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => <option value={value}>{children}</option>,
}));

function Harness() {
  const [school, setSchool] = React.useState<SchoolOption | null>(null);
  const [unlisted, setUnlisted] = React.useState("");
  return (
    <>
      <SchoolPicker
        value={school?.id ?? ""}
        selectedSchool={school}
        onChange={setSchool}
        allowUnlisted
        unlistedName={unlisted}
        onUnlistedNameChange={setUnlisted}
      />
      <output data-testid="school-id">{school?.id ?? ""}</output>
      <output data-testid="unlisted">{unlisted}</output>
    </>
  );
}

function HarnessWithExistingUnlisted() {
  const [school, setSchool] = React.useState<SchoolOption | null>(null);
  const [unlisted, setUnlisted] = React.useState("SMA Lama Manual");
  return (
    <>
      <SchoolPicker
        value={school?.id ?? ""}
        selectedSchool={school}
        onChange={setSchool}
        allowUnlisted
        unlistedName={unlisted}
        onUnlistedNameChange={setUnlisted}
      />
      <output data-testid="school-id">{school?.id ?? ""}</output>
      <output data-testid="unlisted">{unlisted}</output>
    </>
  );
}

describe("SchoolPicker", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    searchCalls.length = 0;
    lastSearchKey = "";
    schools = defaultSchools;
    searchError = false;
    hydratedSchool = null;
    hasMore = false;
    fetchNextPage.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps location filters readable in narrow containers", () => {
    render(<Harness />);

    expect(screen.getByTestId("school-picker-location-filters")).toHaveClass(
      "[grid-template-columns:repeat(auto-fit,minmax(9rem,1fr))]",
    );
  });

  it.each([
    [0, "province-1", { province_id: "province-1", city_id: "", school_type: "" }],
    [1, "city-1", { province_id: "", city_id: "city-1", school_type: "" }],
    [2, "SMA", { province_id: "", city_id: "", school_type: "SMA" }],
  ])("searches with only filter %s selected", (index, value, params) => {
    render(<Harness />);
    const select = screen.getAllByRole("combobox")[index as number];
    expect(select).not.toBeDisabled();
    fireEvent.change(select, { target: { value } });
    typeName("negeri");
    expect(searchCalls.at(-1)).toMatchObject({ enabled: true, params: { ...params, q: "negeri", limit: 20 } });
    expect(screen.getByRole("option", { name: /SMA Negeri 1 Jakarta/ })).toBeInTheDocument();
  });

  it("debounces global name search and hides old results while typing", () => {
    render(<Harness />);
    const input = screen.getByPlaceholderText("Nama sekolah");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "negeri" } });
    expect(searchCalls.at(-1)?.enabled).toBe(false);
    act(() => { vi.advanceTimersByTime(300); });
    expect(searchCalls.at(-1)).toMatchObject({ enabled: true, params: { q: "negeri", limit: 20 } });
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "bina" } });
    expect(screen.queryByRole("option", { name: /SMA Negeri/ })).toBeNull();
  });

  it("reflects a keyboard selection immediately and clears its ID on editing", () => {
    render(<Harness />);
    typeName("negeri");
    const input = screen.getByPlaceholderText("Nama sekolah");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    expect(input).toHaveValue("SMA Negeri 1 Jakarta");
    expect(input).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("listbox")).toBeNull();
    fireEvent.change(input, { target: { value: "another" } });
    expect(screen.getByTestId("school-id")).toBeEmptyDOMElement();
  });

  it("loads more matching schools without dropping the query", () => {
    hasMore = true;
    render(<Harness />);
    typeName("negeri");
    fireEvent.click(screen.getByRole("button", { name: "Tampilkan lebih banyak" }));
    expect(fetchNextPage).toHaveBeenCalledOnce();
  });

  it("distinguishes search errors from an empty result", () => {
    searchError = true;
    render(<Harness />);
    typeName("negeri");
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal mencari sekolah");
    expect(screen.queryByText("Sekolah tidak ditemukan.")).toBeNull();
  });

  it("keeps exact NPSN lookup and reflects the selected school when switching modes", () => {
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "NPSN" }));
    const input = screen.getByPlaceholderText("Masukkan NPSN");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "12345678" } });
    fireEvent.click(screen.getByRole("option", { name: /SMA Negeri 1 Jakarta/ }));
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    fireEvent.click(screen.getByRole("button", { name: "Nama" }));
    expect(screen.getByPlaceholderText("Nama sekolah")).toHaveValue("SMA Negeri 1 Jakarta");
  });

  it("reflects legacy type labels in the filter without changing the selected record", () => {
    schools = [{ ...defaultSchools[0], school_types: [" smp "] }];
    render(<Harness />);
    typeName("negeri");
    fireEvent.click(screen.getByRole("option", { name: /SMA Negeri 1 Jakarta/ }));
    expect(screen.getAllByRole("combobox")[2]).toHaveValue("SMP");
    expect(schools[0].school_types).toEqual([" smp "]);
  });

  it("shows an existing school immediately after hydration", () => {
    const { rerender } = render(<SchoolPicker value="school-1" onChange={vi.fn()} />);
    hydratedSchool = defaultSchools[0];
    rerender(<SchoolPicker value="school-1" onChange={vi.fn()} />);
    expect(screen.getByPlaceholderText("Nama sekolah")).toHaveValue(defaultSchools[0].name);
    expect(screen.getByText(defaultSchools[0].name)).toBeInTheDocument();
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("does not search or show suggestions when disabled", () => {
    const { rerender } = render(<SchoolPicker onChange={vi.fn()} />);
    typeName("negeri");
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    rerender(<SchoolPicker disabled onChange={vi.fn()} />);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByPlaceholderText("Nama sekolah")).toBeDisabled();
    expect(searchCalls.at(-1)?.enabled).toBe(false);
  });

  it("Escape dismisses suggestions without choosing a school", () => {
    render(<Harness />);
    typeName("negeri");
    fireEvent.keyDown(screen.getByPlaceholderText("Nama sekolah"), { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByTestId("school-id")).toBeEmptyDOMElement();
  });

  it("activates explicit unlisted fallback without inventing a school", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: "Sekolah tidak ditemukan" }));
    expect(screen.getByTestId("unlisted")).toHaveTextContent("");

    fireEvent.change(screen.getByPlaceholderText("Tulis nama sekolah"), { target: { value: "SMA Baru Manual" } });

    expect(screen.getByTestId("school-id")).toHaveTextContent("");
    expect(screen.getByTestId("unlisted")).toHaveTextContent("SMA Baru Manual");
  });

  it("can switch from existing unlisted fallback back to searchable schools", async () => {
    render(<HarnessWithExistingUnlisted />);

    expect(screen.getByPlaceholderText("Tulis nama sekolah")).toHaveValue("SMA Lama Manual");

    fireEvent.click(screen.getByRole("button", { name: "Nama" }));
    expect(screen.getByTestId("unlisted")).toHaveTextContent("");
    expect(screen.getByPlaceholderText("Nama sekolah")).toBeInTheDocument();

    const [provinceSelect, citySelect, categorySelect] = screen.getAllByRole("combobox");
    fireEvent.change(provinceSelect, { target: { value: "province-1" } });
    fireEvent.change(citySelect, { target: { value: "city-1" } });
    fireEvent.change(categorySelect, { target: { value: "SMA" } });
    fireEvent.focus(screen.getByPlaceholderText("Nama sekolah"));
    fireEvent.change(screen.getByPlaceholderText("Nama sekolah"), { target: { value: "sma negeri" } });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    fireEvent.click(screen.getByText("SMA Negeri 1 Jakarta"));
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    expect(screen.getByTestId("unlisted")).toHaveTextContent("");
  });
});

function typeName(value: string) {
  const input = screen.getByPlaceholderText("Nama sekolah");
  fireEvent.focus(input);
  fireEvent.change(input, { target: { value } });
  act(() => { vi.advanceTimersByTime(300); });
}
