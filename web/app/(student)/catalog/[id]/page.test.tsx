import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Suspense } from "react";
import ProductDetailPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

const BASE_PRODUCT = {
  id: "medal-1",
  type: "medal",
  name: "Medali Akademi",
  price: 75000,
  stock: 12,
  status: "published",
} as Record<string, unknown>;

let product: Record<string, unknown>;

vi.mock("@/lib/hooks/products", () => ({
  useProduct: () => ({
    data: product,
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  }),
}));

vi.mock("@/lib/hooks/orders", () => ({
  useAddToCart: () => ({ mutate: vi.fn(), isPending: false }),
  useCart: () => ({ data: { items: [] } }),
}));

vi.mock("@/stores/cart", () => ({
  useCartStore: (selector: (state: { setCount: ReturnType<typeof vi.fn> }) => unknown) => selector({ setCount: vi.fn() }),
  default: { getState: () => ({ count: 0 }) },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (state: { token: string }) => unknown) => selector({ token: "token" }),
}));

async function renderPage(overrides: Record<string, unknown> = {}) {
  product = { ...BASE_PRODUCT, ...overrides };
  await act(async () => {
    render(
      <Suspense fallback={null}>
        <ProductDetailPage params={Promise.resolve({ id: "medal-1" })} />
      </Suspense>,
    );
  });
}

beforeEach(() => {
  product = { ...BASE_PRODUCT };
});

describe("ProductDetailPage", () => {
  it("shows stock and delivery information for medals", async () => {
    await renderPage();

    expect(await screen.findAllByText(/stok: 12/i)).toHaveLength(2);
    expect(screen.getByText(/dikirim ke alamat/i)).toBeInTheDocument();
  });

  // Descriptions are typed into a plain textarea, so an admin's paragraph break
  // is a bare newline. HTML collapses those, and the storefront ran every
  // paragraph together into one block.
  it("keeps the paragraph breaks the admin typed", async () => {
    await renderPage({ description: "Paragraf pertama.\n\nParagraf kedua." });

    const body = screen.getByText(/Paragraf pertama/);
    expect(body.className).toContain("whitespace-pre-line");
    expect(body.textContent).toContain("\n\nParagraf kedua.");
  });

  // The live catalogue has a record with four consecutive newlines, which
  // pre-line renders as a hole roughly a paragraph tall.
  it("collapses a run of blank lines to a single break", async () => {
    await renderPage({ description: "Pertama.\n\n\n\nKedua." });

    expect(screen.getByText(/Pertama/).textContent).toBe("Pertama.\n\nKedua.");
  });

  it("clamps a long description until the reader asks for the rest", async () => {
    const long = "Buku ini membahas materi olimpiade sains. ".repeat(20);
    await renderPage({ description: long });

    const body = screen.getByText(/Buku ini membahas/);
    expect(body.className).toContain("line-clamp-6");

    await userEvent.click(screen.getByRole("button", { name: /lihat selengkapnya/i }));
    expect(screen.getByText(/Buku ini membahas/).className).not.toContain("line-clamp-6");
  });

  it("leaves a short description unclamped and offers no expander", async () => {
    await renderPage({ description: "Medali emas, diameter 5 cm." });

    expect(screen.getByText(/Medali emas/).className).not.toContain("line-clamp");
    expect(screen.queryByRole("button", { name: /lihat selengkapnya/i })).toBeNull();
  });

  // The stepper goes to 10, so the price shown against a single unit stopped
  // being what the buyer would actually pay.
  it("tracks the subtotal against the chosen quantity", async () => {
    await renderPage();

    const panel = within(screen.getByRole("complementary", { name: /atur jumlah/i }));
    expect(panel.getByText("Rp75.000")).toBeInTheDocument();

    const [, plus] = panel.getAllByRole("button", { name: /jumlah/i });
    await userEvent.click(plus);

    expect(panel.getByText("Rp150.000")).toBeInTheDocument();
  });

  // Specifications answer what a buyer checks first. They used to sit below a
  // description that can run several hundred words, which put them off screen.
  it("places the specifications above the description", async () => {
    await renderPage({
      description: "Deskripsi produk.",
      specs: [{ key: "bahan", label: "Bahan", value: "Kuningan" }],
    });

    const specs = screen.getByRole("heading", { name: /spesifikasi produk/i });
    const desc = screen.getByRole("heading", { name: /^deskripsi$/i });

    expect(specs.compareDocumentPosition(desc) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
