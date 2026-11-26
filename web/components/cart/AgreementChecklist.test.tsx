import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { AgreementChecklist } from "./AgreementChecklist";
import type { Product } from "@/lib/types";

const apiFetchMock = vi.fn();

vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
}));

function renderWithQuery(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

function product(overrides: Partial<Product>): Product {
  return {
    id: "p1",
    type: "exam",
    name: "Try Out UTBK",
    price: 75000,
    agreement_checkboxes: [],
    ...overrides,
  };
}

describe("AgreementChecklist", () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it("renders nothing when no item's product has any configured checkbox, and reports allAgreed=true", async () => {
    apiFetchMock.mockResolvedValue(product({ agreement_checkboxes: [] }));
    const onChange = vi.fn();

    renderWithQuery(
      <AgreementChecklist items={[{ product_id: "p1", name: "Try Out UTBK" }]} onChange={onChange} />,
    );

    await waitFor(() => expect(apiFetchMock).toHaveBeenCalled());
    expect(screen.queryByTestId("agreement-checklist")).not.toBeInTheDocument();
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith({}, true));
  });

  it("renders every configured checkbox and reports allAgreed=false until every one is checked", async () => {
    apiFetchMock.mockResolvedValue(
      product({
        agreement_checkboxes: [
          { id: "c1", title: "Tata tertib", body: "Saya bersedia mengikuti tata tertib ujian." },
          { id: "c2", title: "Refund", body: "Saya memahami kebijakan refund." },
        ],
      }),
    );
    const onChange = vi.fn();

    renderWithQuery(
      <AgreementChecklist items={[{ product_id: "p1", name: "Try Out UTBK" }]} onChange={onChange} />,
    );

    const checkbox1 = await screen.findByLabelText("Tata tertib");
    const checkbox2 = screen.getByLabelText("Refund");
    expect(screen.getByText("Saya bersedia mengikuti tata tertib ujian.")).toBeInTheDocument();

    await waitFor(() =>
      expect(onChange).toHaveBeenLastCalledWith({ p1: { c1: false, c2: false } }, false),
    );

    fireEvent.click(checkbox1);
    await waitFor(() =>
      expect(onChange).toHaveBeenLastCalledWith({ p1: { c1: true, c2: false } }, false),
    );

    fireEvent.click(checkbox2);
    await waitFor(() =>
      expect(onChange).toHaveBeenLastCalledWith({ p1: { c1: true, c2: true } }, true),
    );
  });

  it("never fetches the same product twice when two items share a product_id", async () => {
    apiFetchMock.mockResolvedValue(product({ agreement_checkboxes: [] }));

    renderWithQuery(
      <AgreementChecklist
        items={[
          { product_id: "p1", name: "Try Out UTBK" },
          { product_id: "p1", name: "Try Out UTBK (dup)" },
        ]}
        onChange={vi.fn()}
      />,
    );

    await waitFor(() => expect(apiFetchMock).toHaveBeenCalled());
    expect(apiFetchMock).toHaveBeenCalledTimes(1);
  });
});
