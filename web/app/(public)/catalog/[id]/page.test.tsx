import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Suspense } from "react";
import ProductDetailPage from "./page";
import { ApiError } from "@/lib/api";
import { useGuestCart } from "@/stores/guest-cart";
import type { Product } from "@/lib/types";

const { routerBack, routerPush } = vi.hoisted(() => ({ routerBack: vi.fn(), routerPush: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: routerPush, back: routerBack }),
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

const { addToCartMutate, toastError, toastSuccess, toastMessage, cartItems, auth, authedHookArgs } = vi.hoisted(() => ({
  authedHookArgs: vi.fn(),
  addToCartMutate: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  toastMessage: vi.fn(),
  cartItems: [] as { product_id: string }[],
  auth: { token: "token" as string | null },
}));

vi.mock("sonner", () => ({
  toast: { error: toastError, success: toastSuccess, message: toastMessage },
}));

vi.mock("@/lib/hooks/orders", () => ({
  useAddToCart: () => ({ mutate: addToCartMutate, isPending: false }),
  useCart: (opts?: { enabled?: boolean; redirectOn401?: boolean }) => (authedHookArgs("cart", opts?.enabled ?? true, opts?.redirectOn401), { data: { items: cartItems } }),
}));

let profile: Record<string, unknown> | undefined;

vi.mock("@/lib/hooks/students", () => ({
  useProfile: (opts?: { enabled?: boolean; redirectOn401?: boolean }) => (authedHookArgs("profile", opts?.enabled ?? true, opts?.redirectOn401), { data: profile }),
}));

vi.mock("@/stores/cart", () => ({
  useCartStore: (selector: (state: { setCount: ReturnType<typeof vi.fn> }) => unknown) => selector({ setCount: vi.fn() }),
  default: { getState: () => ({ count: 0 }) },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (state: { token: string | null }) => unknown) => selector({ token: auth.token }),
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
  cartItems.length = 0;
  addToCartMutate.mockReset();
  toastError.mockReset();
  toastSuccess.mockReset();
  toastMessage.mockReset();
  routerBack.mockReset();
  routerPush.mockReset();
  auth.token = "token";
  useGuestCart.setState({ lines: [] });
  product = { ...BASE_PRODUCT };
  profile = undefined;
});

describe("ProductDetailPage", () => {
  it("shows stock and delivery information for medals", async () => {
    await renderPage();

    expect(await screen.findAllByText(/stok: 12/i)).toHaveLength(2);
    expect(screen.getByText(/dikirim ke alamat/i)).toBeInTheDocument();
  });

  // Going back must land on whatever /catalog URL (filters, search) the
  // reader actually came from — a fixed href="/catalog" link would drop them.
  it("goes back in history instead of a fixed /catalog link", async () => {
    await renderPage();

    await userEvent.click(screen.getByRole("button", { name: /toko/i }));

    expect(routerBack).toHaveBeenCalledTimes(1);
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

  // The checkout gate used to be the first place a student learned their
  // biodata was incomplete. This warns them upfront, on the exam product page.
  describe("exam biodata warning", () => {
    it("shows the inline prompt naming missing fields when biodata is incomplete", async () => {
      profile = { id: "u1", school_id: "s1" }; // grade and dob missing

      await renderPage({ id: "exam-1", type: "exam", name: "Ujian Try Out" });

      expect(
        screen.getByText(/lengkapi biodata sebelum mendaftar/i),
      ).toBeInTheDocument();
      const body = screen.getByText(/kelas, tanggal lahir/i);
      expect(body.textContent).not.toMatch(/sekolah/i);
      expect(screen.getByRole("link", { name: /lengkapi profil/i })).toHaveAttribute(
        "href",
        "/profile",
      );
    });

    it("shows nothing to a guest, who has no profile to complete", async () => {
      auth.token = null;
      await renderPage({ id: "exam-1", type: "exam", name: "Ujian Try Out" });
      expect(screen.queryByText(/lengkapi biodata sebelum mendaftar/i)).toBeNull();
      expect(screen.queryByRole("link", { name: /lengkapi profil/i })).toBeNull();
    });

    it("shows nothing when biodata is complete", async () => {
      profile = { id: "u1", school_id: "s1", grade: 10, dob: "2008-01-01" };

      await renderPage({ id: "exam-1", type: "exam", name: "Ujian Try Out" });

      expect(screen.queryByText(/lengkapi biodata sebelum mendaftar/i)).toBeNull();
      expect(screen.queryByRole("link", { name: /lengkapi profil/i })).toBeNull();
    });

    it("shows nothing for a non-exam product regardless of profile", async () => {
      profile = undefined;

      await renderPage(); // medal, from BASE_PRODUCT

      expect(screen.queryByText(/lengkapi biodata sebelum mendaftar/i)).toBeNull();
    });
  });
  describe("add-to-cart failure", () => {
    const TARGET = "0b6f7c1e-3a52-4c1d-9d0e-6f1b1f2f7a11";

    function failWith(err: ApiError) {
      addToCartMutate.mockImplementation((_vars, opts: { onError: (e: unknown) => void }) => opts.onError(err));
    }

    async function addToCart() {
      await userEvent.click(screen.getByRole("button", { name: /tambah ke keranjang/i }));
    }

    it("explains an owned digital target instead of the raw backend text", async () => {
      failWith(
        new ApiError("digital_target_conflict", "digital product already owned or being purchased", 409, {
          details: { conflicts: [{ product_id: "p1", kind: "course", target_id: TARGET, reason: "owned" }] },
        }),
      );
      await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });

      await addToCart();

      expect(toastError).toHaveBeenCalledTimes(1);
      const [, opts] = toastError.mock.calls[0];
      expect(opts.description).toBe(
        "Kamu sudah memiliki kursus ini.",
      );
      expect(opts.description).not.toContain(TARGET);
    });

    it("tells the student what to do when another order already claims the target", async () => {
      failWith(
        new ApiError("digital_target_conflict", "digital product already owned or being purchased", 409, {
          details: { conflicts: [{ product_id: "p1", kind: "exam", target_id: TARGET, reason: "claimed" }] },
        }),
      );
      await renderPage({ id: "exam-1", type: "exam", name: "Ujian Try Out" });

      await addToCart();

      expect(toastError.mock.calls[0][1].description).toMatch(/pesanan lain yang belum selesai/i);
    });

    it("explains an unlinked digital product", async () => {
      failWith(new ApiError("digital_product_unavailable", "digital product has no targets", 409));
      await renderPage({ id: "exam-1", type: "exam", name: "Ujian Try Out" });

      await addToCart();

      expect(toastError.mock.calls[0][1].description).toMatch(/belum tersedia.*hubungi admin/i);
    });

    it("keeps the server message for other API errors", async () => {
      failWith(new ApiError("invalid_qty", "quantity too high", 400));
      await renderPage();

      await addToCart();

      expect(toastError.mock.calls[0][1].description).toBe("quantity too high");
    });
  });

  describe("exam stock", () => {
    it("shows the remaining quota for a limited exam", async () => {
      await renderPage({ id: "exam-1", type: "exam", name: "Tryout UTBK", stock: 5, stock_limited: true });

      expect(screen.getByText("Stok: 5")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /tambah ke keranjang/i })).toBeEnabled();
    });

    it("shows Habis and disables both purchase buttons for a sold-out limited exam", async () => {
      await renderPage({ id: "exam-1", type: "exam", name: "Tryout UTBK", stock: 0, stock_limited: true });

      expect(screen.getByText("Habis")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /tambah ke keranjang/i })).toBeDisabled();
      expect(screen.getByRole("button", { name: /beli sekarang/i })).toBeDisabled();
    });

    it("keeps View Cart reachable when a sold-out exam is already in the cart", async () => {
      cartItems.push({ product_id: "exam-1" });
      await renderPage({ id: "exam-1", type: "exam", name: "Tryout UTBK", stock: 0, stock_limited: true });

      expect(screen.getByRole("button", { name: /lihat keranjang/i })).toBeEnabled();
    });

    it("shows no stock line for an unlimited exam", async () => {
      await renderPage({ id: "exam-1", type: "exam", name: "Tryout UTBK" });

      expect(screen.queryByText(/stok/i)).toBeNull();
      expect(screen.getByRole("button", { name: /tambah ke keranjang/i })).toBeEnabled();
    });
  });
});

describe("stale token", () => {
  it("opts cart and profile out of the /login redirect", async () => {
    authedHookArgs.mockClear();
    await renderPage();
    expect(authedHookArgs).toHaveBeenCalledWith("cart", true, false);
    expect(authedHookArgs).toHaveBeenCalledWith("profile", true, false);
  });

  it("a 401 through the real hooks never navigates to /login", async () => {
    const { QueryClient, QueryClientProvider } = await import("@tanstack/react-query");
    const { renderHook, waitFor } = await import("@testing-library/react");
    const { useAuthStore } = await vi.importActual<typeof import("@/stores/auth")>("@/stores/auth");
    const { useCart } = await vi.importActual<typeof import("@/lib/hooks/orders")>("@/lib/hooks/orders");
    const { useProfile } = await vi.importActual<typeof import("@/lib/hooks/students")>("@/lib/hooks/students");
    useAuthStore.setState({ token: "stale" });
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "unauthorized", message: "x" } }), { status: 401 }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const loc = { href: "/catalog/medal-1" };
    vi.stubGlobal("location", loc);
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(
      () => [useCart({ redirectOn401: false }), useProfile({ redirectOn401: false })] as const,
      { wrapper },
    );
    await waitFor(() => expect(result.current[0].isError && result.current[1].isError).toBe(true));
    expect(loc.href).toBe("/catalog/medal-1");
    vi.unstubAllGlobals();
  });
});

describe("guest add-to-cart", () => {
  beforeEach(() => {
    auth.token = null;
  });

  const addButton = () => screen.getByRole("button", { name: /tambah ke keranjang/i });

  it("never enables the signed-in cart or profile queries for a guest (their 401 redirects to /login)", async () => {
    authedHookArgs.mockClear();
    await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });
    expect(authedHookArgs).toHaveBeenCalledWith("cart", false, false);
    expect(authedHookArgs).toHaveBeenCalledWith("profile", false, false);
    expect(authedHookArgs).not.toHaveBeenCalledWith("cart", true, false);
    expect(authedHookArgs).not.toHaveBeenCalledWith("profile", true, false);
  });

  it("adds to the guest cart instead of sending the guest to /login", async () => {
    await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });

    await userEvent.click(addButton());

    expect(useGuestCart.getState().lines).toMatchObject([{ product_id: "course-1", qty: 1 }]);
    expect(addToCartMutate).not.toHaveBeenCalled();
    expect(routerPush).not.toHaveBeenCalledWith("/login");
    expect(toastSuccess).toHaveBeenCalledWith(
      "Ditambahkan ke keranjang",
      expect.objectContaining({ action: expect.objectContaining({ label: "Lihat keranjang" }) }),
    );
  });

  it("the toast action opens the cart", async () => {
    await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });
    await userEvent.click(addButton());

    toastSuccess.mock.calls[0][1].action.onClick();

    expect(routerPush).toHaveBeenCalledWith("/cart");
  });

  it("buy now adds and then goes to the cart", async () => {
    await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });

    await userEvent.click(screen.getByRole("button", { name: /beli sekarang/i }));

    expect(useGuestCart.getState().lines).toHaveLength(1);
    expect(routerPush).toHaveBeenCalledWith("/cart");
  });

  it("says so when a digital product is added a second time", async () => {
    await renderPage({ id: "course-1", type: "course", name: "Kursus Fisika" });

    await userEvent.click(addButton());
    await userEvent.click(addButton());

    expect(useGuestCart.getState().lines).toHaveLength(1);
    expect(toastMessage).toHaveBeenCalledWith("Sudah ada di keranjangmu", expect.anything());
  });

  it("caps the physical stepper at the visible stock", async () => {
    await renderPage({ stock: 2 });
    const panel = within(screen.getByRole("complementary", { name: /atur jumlah/i }));
    const [, plus] = panel.getAllByRole("button", { name: /jumlah/i });

    await userEvent.click(plus);
    expect(plus).toBeDisabled();
    await userEvent.click(addButton());

    expect(useGuestCart.getState().lines[0].qty).toBe(2);
  });

  it("reports out of stock when the stock is already in the guest cart", async () => {
    useGuestCart.getState().add(BASE_PRODUCT as unknown as Product, 12);
    await renderPage();

    await userEvent.click(addButton());

    expect(toastError).toHaveBeenCalledWith("Stok habis", expect.anything());
    expect(useGuestCart.getState().lines[0].qty).toBe(12);
  });

  it("reports a failure when the guest cart is full", async () => {
    for (let i = 0; i < 20; i++) {
      useGuestCart.getState().add({ id: `c${i}`, type: "course", name: `K${i}`, price: 1 });
    }
    await renderPage({ id: "course-new", type: "course", name: "Kursus Baru" });

    await userEvent.click(addButton());

    expect(toastError).toHaveBeenCalledWith("Keranjang tamu penuh (maks. 20 item). Masuk untuk melanjutkan.");
    expect(useGuestCart.getState().lines).toHaveLength(20);
  });
});
