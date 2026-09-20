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
  useProvinces: () => ({ data: [{ id: "province-1", name: "DKI JAKARTA" }, { id: "province-2", name: "JAWA TIMUR" }] }),
  useCitiesByProvince: () => ({ data: [{ id: "city-1", province_id: "province-1", name: "KOTA JAKARTA PUSAT" }] }),
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchoolById: () => ({ data: hydratedSchool }),
  useSchoolAutocomplete: (params: Record<string, unknown>, enabled: boolean) => {
    const key = JSON.stringify({ params, enabled });
    if (key !== lastSearchKey) {
      searchCalls.push({ params, enabled });
      lastSearchKey = key;
    }
    return { data: { pages: [{ data: schools }] }, isFetching: false, isError: searchError, hasNextPage: hasMore, fetchNextPage, refetch: vi.fn() };
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

  it.each(["province-2", "_empty_"])("requires a province for cities and resets city when province becomes %s", (nextProvince) => {
    render(<Harness />);
    const [province, city] = screen.getAllByRole("combobox");
    expect(city).toBeDisabled();
    fireEvent.change(province, { target: { value: "province-1" } });
    expect(city).not.toBeDisabled();
    fireEvent.change(city, { target: { value: "city-1" } });
    expect(searchCalls.at(-1)).toMatchObject({ enabled: true, params: { province_id: "province-1", city_id: "city-1", school_type: "" } });
    fireEvent.change(province, { target: { value: nextProvince } });
    expect(city).toHaveValue("_empty_");
    expect(searchCalls.at(-1)?.params.city_id).toBe("");
    if (nextProvince === "_empty_") expect(city).toBeDisabled();
  });

  it.each(["ne", "𠮷a"])("does not search fewer than three characters: %s", (query) => {
    render(<Harness />);
    typeName(query);
    expect(searchCalls.at(-1)?.enabled).toBe(false);
    expect(screen.queryByRole("option", { name: /SMA Negeri/ })).toBeNull();
    expect(screen.getByText("Ketik minimal 3 karakter nama sekolah.")).toBeInTheDocument();
    typeName(query + "g");
    expect(searchCalls.at(-1)?.enabled).toBe(true);
  });

  it.each([[0, "province-2"], [1, "city-1"], [2, "SMK"]])("keeps a selected school until explicitly cleared despite filter %s events", (index, value) => {
    render(<Harness />);
    typeName("negeri");
    fireEvent.click(screen.getByRole("option", { name: /SMA Negeri 1 Jakarta/ }));
    const filters = screen.getAllByRole("combobox");
    fireEvent.change(filters[index as number], { target: { value } });
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    expect(screen.getByRole("group", { name: "Sekolah terpilih" })).toHaveTextContent("SMA Negeri 1 Jakarta");
    expect(filters.every(filter => (filter as HTMLSelectElement).disabled)).toBe(true);
    expect(searchCalls.at(-1)?.enabled).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Ganti sekolah" }));
    expect(screen.getByTestId("school-id")).toBeEmptyDOMElement();
    expect(screen.getAllByRole("combobox")[0]).not.toBeDisabled();
    expect(screen.getAllByRole("combobox")[2]).not.toBeDisabled();
  });

  it("debounces global name search and hides old results while typing", () => {
    render(<Harness />);
    const input = screen.getByPlaceholderText("Nama sekolah");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "negeri" } });
    expect(searchCalls.at(-1)?.enabled).toBe(false);
    act(() => { vi.advanceTimersByTime(499); });
    expect(searchCalls.at(-1)?.enabled).toBe(false);
    act(() => { vi.advanceTimersByTime(1); });
    expect(searchCalls.at(-1)).toMatchObject({ enabled: true, params: { q: "negeri", limit: 20 } });
    expect(screen.getByRole("listbox")).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "bina" } });
    expect(screen.queryByRole("option", { name: /SMA Negeri/ })).toBeNull();
  });

  it("replaces search with one selected school and the clear button restores a focused empty search", () => {
    render(<Harness />);
    typeName("negeri");
    const input = screen.getByPlaceholderText("Nama sekolah");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    expect(input).not.toBeInTheDocument();
    expect(screen.getAllByText("SMA Negeri 1 Jakarta")).toHaveLength(1);
    expect(screen.getByRole("group", { name: "Sekolah terpilih" })).toHaveTextContent("12345678");
    expect(screen.queryByRole("listbox")).toBeNull();
    const clear = screen.getByRole("button", { name: "Ganti sekolah" });
    expect(clear).toHaveFocus();
    fireEvent.click(clear);
    expect(screen.getByTestId("school-id")).toBeEmptyDOMElement();
    expect(screen.queryByRole("group", { name: "Sekolah terpilih" })).toBeNull();
    expect(screen.getByPlaceholderText("Nama sekolah")).toHaveValue("");
    expect(screen.getByPlaceholderText("Nama sekolah")).toHaveFocus();
    expect(screen.getAllByRole("combobox")[2]).toHaveValue("_empty_");
    typeName("bina");
    fireEvent.click(screen.getByRole("option", { name: /SMA Bina Bangsa/ }));
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-2");
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
    expect(screen.queryByPlaceholderText("Nama sekolah")).toBeNull();
    expect(screen.getByRole("group", { name: "Sekolah terpilih" })).toHaveTextContent("SMA Negeri 1 Jakarta");
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
    expect(screen.queryByPlaceholderText("Nama sekolah")).toBeNull();
    expect(screen.getByRole("group", { name: "Sekolah terpilih" })).toHaveTextContent(defaultSchools[0].name);
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

  it("cannot clear a disabled selected school", () => {
    const onChange = vi.fn();
    render(<SchoolPicker value="school-1" selectedSchool={defaultSchools[0]} disabled onChange={onChange} />);
    const clear = screen.getByRole("button", { name: "Ganti sekolah" });
    expect(clear).toBeDisabled();
    fireEvent.click(clear);
    expect(onChange).not.toHaveBeenCalled();
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
      vi.advanceTimersByTime(500);
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
  act(() => { vi.advanceTimersByTime(500); });
}
