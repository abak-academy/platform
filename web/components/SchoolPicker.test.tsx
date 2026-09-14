import * as React from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SchoolPicker } from "./SchoolPicker";
import type { SchoolOption } from "@/lib/types";

const searchCalls: Array<{ params: Record<string, unknown>; enabled: boolean }> = [];
let lastSearchKey = "";
const schools: SchoolOption[] = [
  {
    id: "school-1",
    name: "SMA Negeri 1 Jakarta",
    code: "SMAN1JKT",
    npsn: "12345678",
    category: "SMA",
    kota_name: "KOTA JAKARTA PUSAT",
    provinsi_name: "DKI JAKARTA",
  },
  {
    id: "school-2",
    name: "SMA Bina Bangsa",
    code: "SMABB",
    npsn: "87654321",
    category: "SMA",
    kota_name: "KOTA JAKARTA PUSAT",
    provinsi_name: "DKI JAKARTA",
  },
];

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: [{ id: "province-1", name: "DKI JAKARTA" }] }),
  useCitiesByProvince: () => ({ data: [{ id: "city-1", province_id: "province-1", name: "KOTA JAKARTA PUSAT" }] }),
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchoolById: () => ({ data: null }),
  useSchoolSearch: (params: Record<string, unknown>, enabled: boolean) => {
    const key = JSON.stringify({ params, enabled });
    if (key !== lastSearchKey) {
      searchCalls.push({ params, enabled });
      lastSearchKey = key;
    }
    return { data: enabled ? { data: schools } : { data: [] }, isFetching: false };
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
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("fetches by province and city, then filters typed school name locally", async () => {
    render(<Harness />);

    const [provinceSelect, citySelect, categorySelect] = screen.getAllByRole("combobox");
    fireEvent.change(provinceSelect, { target: { value: "province-1" } });
    fireEvent.change(citySelect, { target: { value: "city-1" } });
    fireEvent.change(categorySelect, { target: { value: "SMA" } });

    const latestBeforeTyping = searchCalls[searchCalls.length - 1];
    expect(latestBeforeTyping.enabled).toBe(true);
    expect(latestBeforeTyping.params).toMatchObject({ province_id: "province-1", city_id: "city-1", category: "SMA", limit: 1000 });
    expect(latestBeforeTyping.params).not.toHaveProperty("q");
    const callsAfterLocationFetch = searchCalls.length;

    fireEvent.change(screen.getByPlaceholderText("Cari nama sekolah"), { target: { value: "negeri" } });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(searchCalls).toHaveLength(callsAfterLocationFetch);
    expect(screen.getByText("SMA Negeri 1 Jakarta")).toBeInTheDocument();
    expect(screen.queryByText("SMA Bina Bangsa")).toBeNull();
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
    expect(screen.getByPlaceholderText("Cari nama sekolah")).toBeInTheDocument();

    const [provinceSelect, citySelect, categorySelect] = screen.getAllByRole("combobox");
    fireEvent.change(provinceSelect, { target: { value: "province-1" } });
    fireEvent.change(citySelect, { target: { value: "city-1" } });
    fireEvent.change(categorySelect, { target: { value: "SMA" } });
    fireEvent.change(screen.getByPlaceholderText("Cari nama sekolah"), { target: { value: "sma negeri" } });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    fireEvent.click(screen.getByText("SMA Negeri 1 Jakarta"));
    expect(screen.getByTestId("school-id")).toHaveTextContent("school-1");
    expect(screen.getByTestId("unlisted")).toHaveTextContent("");
  });
});
