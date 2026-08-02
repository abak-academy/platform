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
  useActivePromoCodes: vi.fn(),
  useCheckout: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
    isError: false,
    data: undefined,
  })),
}));

vi.mock("@/lib/hooks/students", () => ({
  useProfile: vi.fn(),
  useUpdateProfile: vi.fn(),
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
        cart_courier_note_label: "Note for the courier",
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
        cart_address_save: "Save address",
        cart_address_set_primary: "Make this my primary address",
        cart_address_primary_badge: "Primary",
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

import { useCart, useRemoveCartItem, useUpdateCartItemQty, useValidatePromo, useShippingRates, usePatchCart, useActivePromoCodes } from "@/lib/hooks/orders";
import { useProfile, useUpdateProfile } from "@/lib/hooks/students";
import { useProvinces, useCitiesByProvince, useDistrictsByCity } from "@/lib/hooks/regions";

const mockUseCart = useCart as ReturnType<typeof vi.fn>;
const mockUseRemoveCartItem = useRemoveCartItem as ReturnType<typeof vi.fn>;
const mockUseUpdateCartItemQty = useUpdateCartItemQty as ReturnType<typeof vi.fn>;
const mockUseValidatePromo = useValidatePromo as ReturnType<typeof vi.fn>;
const mockUseShippingRates = useShippingRates as ReturnType<typeof vi.fn>;
const mockUsePatchCart = usePatchCart as ReturnType<typeof vi.fn>;
const mockUseActivePromoCodes = useActivePromoCodes as ReturnType<typeof vi.fn>;
const mockUseProfile = useProfile as ReturnType<typeof vi.fn>;
const mockUseUpdateProfile = useUpdateProfile as ReturnType<typeof vi.fn>;
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

    mockUseActivePromoCodes.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    });

    mockUseUpdateProfile.mockReturnValue({
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
      // The picker starts collapsed with a prompt — shipping is a real charge and
      // nothing is chosen on the buyer's behalf — so open it before the options
      // exist. Query by role rather than free text: each row renders a
      // decorative carrier monogram with the same letters (aria-hidden, so
      // assistive tech still hears the courier once).
      expect(screen.getByRole("button", { expanded: false })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { expanded: false }));

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

  // The note has no column of its own — it rides inside the shipping_address
  // JSONB. If it is not attached to the same PATCH that stores the courier, the
  // buyer types "titip di satpam" and nothing ever reaches the warehouse.
  it("sends the courier note with the chosen rate", async () => {
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
      expect(screen.getByLabelText(/note for the courier/i)).toBeInTheDocument();
    });
    await user.type(screen.getByLabelText(/note for the courier/i), "Titip di satpam");

    await user.click(screen.getByRole("button", { expanded: false }));
    await user.click(screen.getByRole("radio", { name: /jne/i }));

    await waitFor(() => {
      expect(patchCartMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          courier: "jne",
          shipping_address: expect.objectContaining({ catatan: "Titip di satpam" }),
        })
      );
    });
  });

  // The order used to store only region IDs, so the admin order detail showed a
  // street and a postcode with no city, province or district anywhere.
  it("snapshots the region names onto the order, not just their ids", async () => {
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
      expect(screen.getByRole("button", { expanded: false })).toBeInTheDocument();
    });
    await user.click(screen.getByRole("button", { expanded: false }));
    await user.click(screen.getByRole("radio", { name: /jne/i }));

    await waitFor(() => {
      expect(patchCartMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          shipping_address: expect.objectContaining({
            provinsi_id: "prov1",
            provinsi: "Jawa Barat",
            kota: "Bandung",
            kecamatan: "Cibadak",
          }),
        })
      );
    });
  });

  it("disables Save address until province/city/district are all selected", async () => {
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

    const saveButton = await screen.findByRole("button", { name: /save address/i });
    expect(saveButton).toBeDisabled();
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

    await user.click(await screen.findByRole("button", { expanded: false }));

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
  // The buyer's profile carries no postal code, so they type that last field by
  // hand — the sequence that produced the white screen on staging.
  function mockProfileWithoutPostalCode() {
    mockUseProfile.mockReturnValue({
      data: {
        id: "user1",
        name: "Budi Test",
        phone: "081200000000",
        alamat_domisili: "Jl. Contoh No. 1",
        provinsi_id: "prov1",
        kota_id: "city1",
        kecamatan_id: "dist1",
        kode_pos: "",
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
        shipping_cost: 8000,
        total: 108000,
        items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
  }

  it("keeps the address form open while the last field is being typed", async () => {
    const user = userEvent.setup();
    mockProfileWithoutPostalCode();

    renderWithQueryClient(<CartPage />);

    const kodePos = await screen.findByLabelText("Postal Code");
    await user.type(kodePos, "1");

    // Every field is now non-empty. The form must not collapse under the buyer
    // mid-edit — only the explicit action closes it.
    expect(screen.getByText("Shipping Address")).toBeInTheDocument();
    expect(kodePos).toHaveValue("1");
  });

  it("reopens the address form from the summary without crashing", async () => {
    const user = userEvent.setup();
    mockProfileWithoutPostalCode();

    renderWithQueryClient(<CartPage />);

    await user.type(await screen.findByLabelText("Postal Code"), "15310");
    // The same action sits in the form and in the courier block; either finishes
    // the address.
    await user.click(screen.getByRole("button", { name: "Save address" }));

    const edit = await screen.findByRole("button", { name: "cart_address_change" });
    await user.click(edit);

    // Remounting the form over the address it had itself emitted used to close a
    // parent/child effect loop and throw "Maximum update depth exceeded".
    expect(await screen.findByText("Shipping Address")).toBeInTheDocument();
    expect(screen.getByLabelText("Postal Code")).toHaveValue("15310");
  });

  it("saves the address onto the order when the buyer saves, before any courier is picked", async () => {
    const user = userEvent.setup();
    const patchCartMutate = vi.fn();
    mockProfileWithoutPostalCode();
    mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });

    renderWithQueryClient(<CartPage />);

    await user.type(await screen.findByLabelText("Postal Code"), "15310");
    await user.click(screen.getByRole("button", { name: "Save address" }));

    // Without this the address only reached the order as a passenger on the
    // courier choice, so abandoning the cart first threw it away.
    await waitFor(() => {
      expect(patchCartMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          shipping_address: expect.objectContaining({ kode_pos: "15310", penerima: "Budi Test" }),
        })
      );
    });
    expect(patchCartMutate.mock.calls[0][0].courier).toBeUndefined();
  });

  it("writes the address to the profile only when the buyer asks for it", async () => {
    const user = userEvent.setup();
    const updateProfileMutate = vi.fn();
    mockProfileWithoutPostalCode();
    mockUseUpdateProfile.mockReturnValue({ mutate: updateProfileMutate, isPending: false, isError: false });

    renderWithQueryClient(<CartPage />);
    await user.type(await screen.findByLabelText("Postal Code"), "15310");

    // The profile has no postal code, so the box defaults on.
    const primary = screen.getByRole("checkbox", { name: /primary address/i });
    expect(primary).toBeChecked();

    await user.click(primary);
    await user.click(screen.getByRole("button", { name: "Save address" }));
    expect(updateProfileMutate).not.toHaveBeenCalled();

    // Reopening offers again — the profile still has no address to lose, so the
    // box comes back ticked rather than remembering a decision about a profile
    // state that has not changed.
    await user.click(screen.getByRole("button", { name: "cart_address_change" }));
    expect(screen.getByRole("checkbox", { name: /primary address/i })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Save address" }));

    await waitFor(() => {
      expect(updateProfileMutate).toHaveBeenCalledWith(
        expect.objectContaining({ kode_pos: "15310", address: "Jl. Contoh No. 1" })
      );
    });
  });

  // The phone reached the order's shipping_address but was omitted from the
  // profile payload, so ticking "make this my primary address" saved every
  // field except the phone — silently, because nothing errored.
  it("carries the phone number to the profile, not only to the order", async () => {
    const user = userEvent.setup();
    const updateProfileMutate = vi.fn();
    mockProfileWithoutPostalCode();
    mockUseUpdateProfile.mockReturnValue({ mutate: updateProfileMutate, isPending: false, isError: false });

    renderWithQueryClient(<CartPage />);
    await user.type(await screen.findByLabelText("Postal Code"), "15310");

    expect(screen.getByRole("checkbox", { name: /primary address/i })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Save address" }));

    await waitFor(() => {
      expect(updateProfileMutate).toHaveBeenCalledWith(
        expect.objectContaining({ phone: "081200000000" })
      );
    });
  });

  it("leaves the primary box unticked when the profile already holds an address", async () => {
    mockUseCart.mockReturnValue({
      data: {
        id: "o1", student_id: "s1", status: "cart", subtotal: 100000,
        discount: 0, shipping_cost: 0, total: 100000, items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    // A full profile address is one the buyer set deliberately; a single order
    // shipped elsewhere must not silently replace it.
    mockUseProfile.mockReturnValue({
      data: {
        id: "user1", name: "Budi Test", phone: "081200000000",
        alamat_domisili: "Jl. Contoh No. 1",
        provinsi_id: "prov1", kota_id: "city1", kecamatan_id: "dist1", kode_pos: "40123",
      },
      isLoading: false,
      isError: false,
    });

    renderWithQueryClient(<CartPage />);

    await waitFor(() => {
      expect(screen.getByRole("checkbox", { name: /primary address/i })).not.toBeChecked();
    });
  });

  it("marks the summary as the primary address only when it matches the profile", async () => {
    const user = userEvent.setup();
    mockProfileWithoutPostalCode();

    renderWithQueryClient(<CartPage />);

    // The profile carries no postal code, so "15310" makes this address differ
    // from the saved one.
    await user.type(await screen.findByLabelText("Postal Code"), "15310");
    await user.click(screen.getByRole("button", { name: "Save address" }));

    expect(await screen.findByRole("button", { name: "cart_address_change" })).toBeInTheDocument();
    expect(screen.queryByText("Primary")).toBeNull();
  });

  it("marks the summary as primary when the address is the profile's own", async () => {
    const user = userEvent.setup();

    mockUseCart.mockReturnValue({
      data: {
        id: "o1", student_id: "s1", status: "cart", subtotal: 100000,
        discount: 0, shipping_cost: 0, total: 100000, items: [physicalItem],
      } as Order,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });

    mockUseProfile.mockReturnValue({
      data: {
        id: "user1", name: "Budi Test", phone: "081200000000",
        alamat_domisili: "Jl. Contoh No. 1",
        provinsi_id: "prov1", kota_id: "city1", kecamatan_id: "dist1", kode_pos: "40123",
      },
      isLoading: false,
      isError: false,
    });

    renderWithQueryClient(<CartPage />);

    await user.click(await screen.findByRole("button", { name: "Save address" }));

    expect(await screen.findByText("Primary")).toBeInTheDocument();
  });

  // B-1 frontend (FR-5..FR-7): the promo apply/clear patch is dedicated and
  // separate from the address/courier patches, and the displayed discount is
  // always the server's, never an optimistic client value.
  describe("promo apply/clear", () => {
    const digitalOnlyCart = (overrides: Partial<Order> = {}): Order =>
      ({
        id: "o1",
        student_id: "s1",
        status: "cart",
        subtotal: 100000,
        discount: 0,
        shipping_cost: 0,
        total: 100000,
        items: [digitalItem],
        ...overrides,
      }) as Order;

    it('pressing "Pakai" with a valid code issues a PATCH whose body contains promo_code', async () => {
      const user = userEvent.setup();
      const patchCartMutate = vi.fn();

      mockUseCart.mockReturnValue({ data: digitalOnlyCart(), isLoading: false, isError: false, refetch: vi.fn() });
      mockUseValidatePromo.mockReturnValue({
        mutate: vi.fn((_input, options) => {
          options?.onSuccess?.({ code: "HEMAT10", discount: 10000, final_total: 90000 });
        }),
        isPending: false,
        data: undefined,
        isError: false,
      });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });

      renderWithQueryClient(<CartPage />);

      await user.type(screen.getByPlaceholderText("Masukkan kode promo"), "HEMAT10");
      await user.click(screen.getByRole("button", { name: "Pakai" }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalled();
      });
      const [body] = patchCartMutate.mock.calls[0];
      expect(body).toHaveProperty("promo_code", "HEMAT10");
    });

    it("a courier selection issues a PATCH whose body does not contain the promo_code key at all", async () => {
      const user = userEvent.setup();
      const patchCartMutate = vi.fn();

      mockUseCart.mockReturnValue({
        data: { ...digitalOnlyCart(), items: [physicalItem], promo_code_id: "promo-1", discount: 10000 },
        isLoading: false,
        isError: false,
        refetch: vi.fn(),
      });
      mockUseShippingRates.mockReturnValue({ mutate: vi.fn(), isPending: false, data: mockRates, isError: false });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });

      renderWithQueryClient(<CartPage />);

      await user.click(await screen.findByRole("button", { expanded: false }));
      await user.click(screen.getByRole("radio", { name: /jne/i }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalledWith(expect.objectContaining({ courier: "jne" }));
      });
      const [body] = patchCartMutate.mock.calls[0];
      expect(body).not.toHaveProperty("promo_code");
    });

    it('the clear action issues promo_code: ""', async () => {
      const user = userEvent.setup();
      const patchCartMutate = vi.fn();

      mockUseCart.mockReturnValue({
        data: digitalOnlyCart({ promo_code_id: "promo-1", discount: 10000, total: 90000 }),
        isLoading: false,
        isError: false,
        refetch: vi.fn(),
      });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });

      renderWithQueryClient(<CartPage />);

      await user.click(await screen.findByRole("button", { name: "Hapus" }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalledWith(expect.objectContaining({ promo_code: "" }));
      });
    });

    it("a rejected promo leaves the displayed discount at the server's value and shows the error, never an optimistic value", async () => {
      const user = userEvent.setup();

      mockUseCart.mockReturnValue({ data: digitalOnlyCart(), isLoading: false, isError: false, refetch: vi.fn() });
      mockUseValidatePromo.mockReturnValue({
        mutate: vi.fn((_input, options) => {
          options?.onError?.(new Error("invalid promo"));
        }),
        isPending: false,
        data: undefined,
        isError: false,
      });
      mockUsePatchCart.mockReturnValue({ mutate: vi.fn(), isPending: false, isError: false });

      renderWithQueryClient(<CartPage />);

      await user.type(screen.getByPlaceholderText("Masukkan kode promo"), "EXPIRED");
      await user.click(screen.getByRole("button", { name: "Pakai" }));

      await waitFor(() => {
        expect(screen.getByText("Promo invalid")).toBeInTheDocument();
      });
      // discount stays at the cart's own value (0) — no optimistic discount row
      // is rendered, and no "Promo diterapkan" badge appears.
      expect(screen.queryByText(/Promo diterapkan/)).not.toBeInTheDocument();
      expect(screen.queryByText("Discount")).not.toBeInTheDocument();
    });

    // FR-14: picking a listed promo must go through the exact same apply
    // path as typing it and pressing "Pakai" — same validate call, same PATCH
    // shape — so a second, divergent apply mechanism would fail this test.
    it("selecting a listed active promo issues the same validate call and PATCH shape as typing + Pakai", async () => {
      const user = userEvent.setup();
      const validatePromoMutate = vi.fn((_input, options) => {
        options?.onSuccess?.({ code: "HEMAT10", discount: 10000, final_total: 90000 });
      });
      const patchCartMutate = vi.fn();

      mockUseCart.mockReturnValue({ data: digitalOnlyCart(), isLoading: false, isError: false, refetch: vi.fn() });
      mockUseValidatePromo.mockReturnValue({
        mutate: validatePromoMutate,
        isPending: false,
        data: undefined,
        isError: false,
      });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });
      mockUseActivePromoCodes.mockReturnValue({
        data: [
          {
            code: "HEMAT10",
            discount_percent: 10,
            discount_amount: null,
            min_order_amount: null,
            max_discount_amount: null,
            expires_at: null,
          },
        ],
        isLoading: false,
        isError: false,
      });

      renderWithQueryClient(<CartPage />);

      await user.click(await screen.findByRole("button", { name: /HEMAT10/i }));

      await waitFor(() => {
        expect(validatePromoMutate).toHaveBeenCalledWith(
          expect.objectContaining({ code: "HEMAT10", orderId: "o1" }),
          expect.anything()
        );
      });
      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalled();
      });
      const [body] = patchCartMutate.mock.calls[0];
      expect(body).toHaveProperty("promo_code", "HEMAT10");
      expect(body).toHaveProperty("orderId", "o1");

      // The manual apply path issues the identical shape — proving the listed
      // promo did not invent a second mechanism. The input already holds the
      // selected code (selectedCode filled it), so pressing "Pakai" again
      // re-runs the exact same handler with the same argument.
      expect(screen.getByPlaceholderText("Masukkan kode promo")).toHaveValue("HEMAT10");
      patchCartMutate.mockClear();
      validatePromoMutate.mockClear();
      await user.click(screen.getByRole("button", { name: "Pakai" }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalled();
      });
      const [manualBody] = patchCartMutate.mock.calls[0];
      expect(manualBody).toEqual(body);
    });

    // FR-14 is additive: an empty list and a failing list request must both
    // leave the manual "Pakai" path fully functional.
    it("manual promo entry keeps working when the active-promo list is empty", async () => {
      const user = userEvent.setup();
      const patchCartMutate = vi.fn();
      const validatePromoMutate = vi.fn((_input, options) => {
        options?.onSuccess?.({ code: "MANUAL5", discount: 5000, final_total: 95000 });
      });

      mockUseCart.mockReturnValue({ data: digitalOnlyCart(), isLoading: false, isError: false, refetch: vi.fn() });
      mockUseValidatePromo.mockReturnValue({
        mutate: validatePromoMutate,
        isPending: false,
        data: undefined,
        isError: false,
      });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });
      mockUseActivePromoCodes.mockReturnValue({ data: [], isLoading: false, isError: false });

      renderWithQueryClient(<CartPage />);

      await user.type(screen.getByPlaceholderText("Masukkan kode promo"), "MANUAL5");
      await user.click(screen.getByRole("button", { name: "Pakai" }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalled();
      });
      const [body] = patchCartMutate.mock.calls[0];
      expect(body).toHaveProperty("promo_code", "MANUAL5");
    });

    it("manual promo entry keeps working when the active-promo list request fails", async () => {
      const user = userEvent.setup();
      const patchCartMutate = vi.fn();
      const validatePromoMutate = vi.fn((_input, options) => {
        options?.onSuccess?.({ code: "MANUAL5", discount: 5000, final_total: 95000 });
      });

      mockUseCart.mockReturnValue({ data: digitalOnlyCart(), isLoading: false, isError: false, refetch: vi.fn() });
      mockUseValidatePromo.mockReturnValue({
        mutate: validatePromoMutate,
        isPending: false,
        data: undefined,
        isError: false,
      });
      mockUsePatchCart.mockReturnValue({ mutate: patchCartMutate, isPending: false, isError: false });
      mockUseActivePromoCodes.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error("network error"),
      });

      renderWithQueryClient(<CartPage />);

      await user.type(screen.getByPlaceholderText("Masukkan kode promo"), "MANUAL5");
      await user.click(screen.getByRole("button", { name: "Pakai" }));

      await waitFor(() => {
        expect(patchCartMutate).toHaveBeenCalled();
      });
      const [body] = patchCartMutate.mock.calls[0];
      expect(body).toHaveProperty("promo_code", "MANUAL5");
    });
  });
});
