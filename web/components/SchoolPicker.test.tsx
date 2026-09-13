import * as React from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SchoolPicker } from "./SchoolPicker";
import type { SchoolOption } from "@/lib/types";

const searchCalls: Array<{ params: Record<string, unknown>; enabled: boolean }> = [];
const schools: SchoolOption[] = [
  {
    id: "school-1",
    name: "SMA Negeri 1 Jakarta",
    code: "SMAN1JKT",
    npsn: "12345678",
    category: "SMA",
    city_name: "KOTA JAKARTA PUSAT",
    province_name: "DKI JAKARTA",
  },
];

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: [{ id: "province-1", name: "DKI JAKARTA" }] }),
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchoolById: () => ({ data: null }),
  useSchoolSearch: (params: Record<string, unknown>, enabled: boolean) => {
    searchCalls.push({ params, enabled });
    return { data: enabled ? { data: schools, next_cursor: "" } : { data: [], next_cursor: "" }, isFetching: false };
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

describe("SchoolPicker", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    searchCalls.length = 0;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("searches by province, optional category, and name without loading all schools", async () => {
    render(<Harness />);

    fireEvent.change(screen.getAllByRole("combobox")[0], { target: { value: "province-1" } });
    fireEvent.change(screen.getByPlaceholderText("Cari nama sekolah"), { target: { value: "sma negeri" } });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    const latest = searchCalls[searchCalls.length - 1];
    expect(latest.enabled).toBe(true);
    expect(latest.params).toMatchObject({ province_id: "province-1", q: "sma negeri", limit: 20 });
    expect(latest.params).not.toHaveProperty("cursor");
    expect(screen.getByText("SMA Negeri 1 Jakarta")).toBeInTheDocument();
  });

  it("activates explicit unlisted fallback without inventing a school", () => {
    render(<Harness />);

    fireEvent.click(screen.getByRole("button", { name: "Sekolah tidak ditemukan" }));
    expect(screen.getByTestId("unlisted")).toHaveTextContent("");

    fireEvent.change(screen.getByPlaceholderText("Tulis nama sekolah"), { target: { value: "SMA Baru Manual" } });

    expect(screen.getByTestId("school-id")).toHaveTextContent("");
    expect(screen.getByTestId("unlisted")).toHaveTextContent("SMA Baru Manual");
  });
});
