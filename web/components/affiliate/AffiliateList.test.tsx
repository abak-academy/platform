import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import { AffiliateList } from "./AffiliateList";

interface Row {
  id: string;
  label: string;
}

const columns = [{ key: "label", header: "Label", cell: (r: Row) => r.label }];

function query(partial: Record<string, unknown>) {
  return partial as unknown as UseQueryResult<{ data: Row[]; page: number; per_page: number; total: number }>;
}

function renderList(q: UseQueryResult<{ data: Row[]; page: number; per_page: number; total: number }>, page = 1, onPageChange = vi.fn()) {
  render(<AffiliateList query={q} columns={columns} rowKey={(r) => r.id} page={page} onPageChange={onPageChange} />);
  return onPageChange;
}

describe("AffiliateList", () => {
  it("shows loading", () => {
    renderList(query({ isLoading: true, isError: false, data: undefined }));
    expect(screen.getByText("Memuat…")).toBeInTheDocument();
  });

  it("shows an error with a working retry", () => {
    const refetch = vi.fn();
    renderList(query({ isLoading: false, isError: true, data: undefined, refetch }));

    fireEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows the empty copy", () => {
    renderList(query({ isLoading: false, isError: false, data: { data: [], page: 1, per_page: 20, total: 0 } }));
    expect(screen.getByText("Belum ada data.")).toBeInTheDocument();
  });

  it("treats a null list as empty instead of crashing", () => {
    renderList(query({ isLoading: false, isError: false, data: { data: null, page: 1, per_page: 20, total: 0 } }));
    expect(screen.getByText("Belum ada data.")).toBeInTheDocument();
  });

  it("renders rows and pages forward", () => {
    const data = { data: [{ id: "1", label: "Baris satu" }], page: 1, per_page: 20, total: 45 };
    const onPageChange = renderList(query({ isLoading: false, isError: false, data }));

    expect(screen.getByText("Baris satu")).toBeInTheDocument();
    expect(screen.getByText("Hal. 1 dari 3 · 45 data")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });
});
