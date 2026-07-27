import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CartPage from "./page";
import type { Order, OrderItem } from "@/lib/types";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/cart",
}));

// Mock the hooks
vi.mock("@/lib/hooks/orders", () => ({
  useCart: vi.fn(),
  useRemoveCartItem: vi.fn(),
  useUpdateCartItemQty: vi.fn(),
  useValidatePromo: vi.fn(),
  useShippingRates: vi.fn(),
  usePatchCart: vi.fn(),
  useCheckout: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
    isError: false,
    data: undefined,
  })),
}));

vi.mock("@/lib/hooks/students", () => ({
  useProfile: vi.fn(),
}));

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: vi.fn(),
  useCitiesByProvince: vi.fn(),
  useDistrictsByCity: vi.fn(),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const dict: Record<string, string> = {
        cart_continue: "Continue shopping",
        cart_title: "Cart",
        cart_item_count: "{n} items",
        cart_order_summary: "Order Summary",
        cart_subtotal: "Subtotal",
        cart_discount: "Discount",
        cart_total: "Total",
        cart_secure_payment: "Secure payment",
        cart_empty_title: "Cart is empty",
        cart_empty_desc: "Browse and add items",
        cart_view_catalog: "View Catalog",
        cart_load_failed: "Failed to load cart",
        cart_promo_invalid: "Promo invalid",
        retry: "Retry",
        cart_shipping_title: "Shipping Address",
        cart_check_shipping_cost: "Check shipping cost",
        cart_shipping_form_error: "Please fill in all required fields",
        order_shipping: "Shipping Cost",
        select_province: "Select Province",
        select_city: "Select City",
        select_district: "Select District",
        students_field_provinsi: "Province",
        students_field_kota: "City",
        students_field_kecamatan: "District",
        students_field_kode_pos: "Postal Code",
        students_field_kode_pos_placeholder: "E.g., 40123",
        cart_shipping_options: "Shipping Options",
        cart_shipping_error: "Unable to calculate",
        update: "Update",
        checkout_process: "Process",
      };
      return dict[key] || key;
    },
    lang: "id",
  }),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({}),
}));

// Mock sonner for toast notifications
vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

import { useCart, useRemoveCartItem, useUpdateCartItemQty, useValidatePromo, useShippingRates, usePatchCart } from "@/lib/hooks/orders";
import { useProfile } from "@/lib/hooks/students";
import { useProvinces, useCitiesByProvince, useDistrictsByCity } from "@/lib/hooks/regions";

const mockUseCart = useCart as ReturnType<typeof vi.fn>;
const mockUseRemoveCartItem = useRemoveCartItem as ReturnType<typeof vi.fn>;
const mockUseUpdateCartItemQty = useUpdateCartItemQty as ReturnType<typeof vi.fn>;
const mockUseValidatePromo = useValidatePromo as ReturnType<typeof vi.fn>;
const mockUseShippingRates = useShippingRates as ReturnType<typeof vi.fn>;
const mockUsePatchCart = usePatchCart as ReturnType<typeof vi.fn>;
const mockUseProfile = useProfile as ReturnType<typeof vi.fn>;
const mockUseProvinces = useProvinces as ReturnType<typeof vi.fn>;
const mockUseCitiesByProvince = useCitiesByProvince as ReturnType<typeof vi.fn>;
const mockUseDistrictsByCity = useDistrictsByCity as ReturnType<typeof vi.fn>;

const digitalItem: OrderItem = {
  id: "i1",
  order_id: "o1",
  product_id: "p1",
  product_type: "course",
  name: "Course Item",
  unit_price: 100000,
  qty: 1,
  jumlah: 100000,
  weight_grams: 0,
};

const physicalItem: OrderItem = {
  id: "i2",
  order_id: "o1",
  product_id: "p2",
  product_type: "book",
  name: "Book Item",
  unit_price: 50000,
  qty: 2,
  jumlah: 100000,
  weight_grams: 500,
};

const mockProvinces = [
  { id: "prov1", name: "Jawa Barat", code: "JB" },
];

const mockCities = [
  { id: "city1", name: "Bandung", code: "BD" },
];

const mockDistricts = [
  { id: "dist1", name: "Cibadak", code: "CB" },
];

const mockRates = [
  {
    courier: "jne",
    service: "OKE",
    estimated_days: 2,
    price: 50000,
  },
];

function renderWithQueryClient(component: React.ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>{component}</QueryClientProvider>
  );
}

describe("CartPage with Shipping", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockUseRemoveCartItem.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    });

    mockUseUpdateCartItemQty.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    });

    mockUseValidatePromo.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      data: undefined,
      isError: false,
    });

    mockUseShippingRates.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      data: undefined,
      isError: false,
    });

    mockUsePatchCart.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      isError: false,
    });

    mockUseProfile.mockReturnValue({
      data: {
        id: "user1",
        name: "Test User",
        provinsi_id: "prov1",
        kota_id: "city1",
        kecamatan_id: "dist1",
        kode_pos: "40123",
      },
      isLoading: false,
      isError: false,
    });

    mockUseProvinces.mockReturnValue({
      data: mockProvinces,
      isLoading: false,
    });

    mockUseCitiesByProvince.mockReturnValue({
      data: mockCities,
      isLoading: false,
    });

    mockUseDistrictsByCity.mockReturnValue({
      data: mockDistricts,
      isLoading: false,
    });
  });

  it("does not render shipping section for digital-only cart", async () => {
    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 0,
        total: 100000,
        items: [digitalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    renderWithQueryClient(<CartPage />);

    await waitFor(() => {
      expect(screen.queryByText(/Shipping Address/i)).not.toBeInTheDocument();
    });
  });

  it("renders shipping section for cart with book items", async () => {
    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 200000,
        discount: 0,
        shipping_cost: 0,
        total: 200000,
        items: [digitalItem, physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    renderWithQueryClient(<CartPage />);

    await waitFor(() => {
      expect(screen.getByText(/Shipping Address/i)).toBeInTheDocument();
    });
  });

  it("shows shipping cost in order summary when shipping_cost > 0", async () => {
    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 50000,
        total: 150000,
        items: [physicalItem],
        selected_courier: "jne",
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    renderWithQueryClient(<CartPage />);

    await waitFor(() => {
      // Check that the shipping cost row is present
      expect(screen.getByText("Shipping Cost")).toBeInTheDocument();
    });
  });

  it("calls usePatchCart when courier is selected", async () => {
    const user = userEvent.setup();
    const patchCartMutate = vi.fn();

    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 0,
        total: 100000,
        items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    mockUseShippingRates.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      data: mockRates,
      isError: false,
    });

    mockUsePatchCart.mockReturnValue({
      mutate: patchCartMutate,
      isPending: false,
      isError: false,
    });

    renderWithQueryClient(<CartPage />);

    await waitFor(() => {
      // Query by role, not free text: the rate row also renders a decorative
      // carrier monogram carrying the same letters (aria-hidden, so assistive
      // tech still hears the courier once), which makes getByText ambiguous.
      expect(screen.getByRole("radio", { name: /jne/i })).toBeInTheDocument();
    });

    const jneOption = screen.getByRole("radio", { name: /jne/i });
    await user.click(jneOption);

    await waitFor(() => {
      expect(patchCartMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          orderId: "o1",
          courier: "jne",
          service: "OKE",
          shipping_cost: 50000,
          province_id: "prov1",
          city_id: "city1",
          district_id: "dist1",
          kode_pos: "40123",
        })
      );
    });
  });

  it("disables Check shipping cost until province/city/district are all selected", async () => {
    // Profile only has a postal code — province/city/district are unset, as
    // happens for a student who never completed their address profile.
    mockUseProfile.mockReturnValue({
      data: {
        id: "user1",
        name: "Test User",
        provinsi_id: null,
        kota_id: null,
        kecamatan_id: null,
        kode_pos: "40123",
      },
      isLoading: false,
      isError: false,
    });

    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 0,
        total: 100000,
        items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    renderWithQueryClient(<CartPage />);

    const checkButton = await screen.findByRole("button", { name: /check shipping cost/i });
    expect(checkButton).toBeDisabled();
  });

  it("selecting the second same-carrier service persists that rate, not the first", async () => {
    const user = userEvent.setup();
    const patchCartMutate = vi.fn();

    const twoJneServices = [
      { courier: "jne", service: "REG", estimated_days: 3, price: 15000 },
      { courier: "jne", service: "YES", estimated_days: 1, price: 30000 },
    ];

    mockUseCart.mockReturnValue({
      data: {
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 0,
        total: 100000,
        items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    mockUseShippingRates.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      data: twoJneServices,
      isError: false,
    });

    mockUsePatchCart.mockReturnValue({
      mutate: patchCartMutate,
      isPending: false,
      isError: false,
    });

    renderWithQueryClient(<CartPage />);

    const options = await screen.findAllByRole("radio", { name: /jne/i });
    expect(options).toHaveLength(2);

    // Address the rows by service rather than position: rates are grouped by
    // delivery speed and sorted by price inside each group, so DOM order no
    // longer follows the order they arrived in.
    const yes = screen.getByRole("radio", { name: /YES/i });
    const reg = screen.getByRole("radio", { name: /REG/i });

    // Selecting YES (30000) must not persist REG (15000).
    await user.click(yes);

    await waitFor(() => {
      expect(patchCartMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          courier: "jne",
          service: "YES",
          shipping_cost: 30000,
        })
      );
    });
    expect(patchCartMutate).not.toHaveBeenCalledWith(
      expect.objectContaining({ service: "REG" })
    );

    // Choosing collapses the list to the chosen option, so the summary is what
    // proves the right same-carrier service stuck. Asserting aria-checked on the
    // rows would inspect nodes that are no longer mounted.
    await waitFor(() => {
      expect(screen.queryAllByRole("radio")).toHaveLength(0);
    });
    expect(screen.getByRole("button", { name: /YES/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /REG/i })).toBeNull();
  });
});
