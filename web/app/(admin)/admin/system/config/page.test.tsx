import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import ConfigPage from "./page";

const translationMap: Record<string, string> = {
  config_shipping_origin: "Asal Pengiriman",
  config_general_kode_pos_placeholder: "Masukkan kode pos",
  config_shipping_latitude: "Lintang pengirim",
  config_shipping_longitude: "Bujur pengirim",
  config_shipping_coordinate_hint: "Titik jemput pengirim untuk booking Biteship; wajib untuk Lion Parcel. Urutan: lintang, lalu bujur. Isi keduanya atau kosongkan keduanya.",
  config_shipping_settings: "Pengaturan Pengiriman",
  config_shipping_biteship_key: "API Key Biteship",
  config_shipping_key_placeholder: "Isi API key Biteship",
  config_shipping_webhook_secret: "Webhook Secret Biteship",
  config_shipping_webhook_secret_placeholder: "Isi webhook secret Biteship",
  config_title: "Konfigurasi Sistem",
  config_subtitle: "Pengaturan platform dan fitur global.",
  save: "Simpan",
  config_payment_mask_hint: "Biarkan *** untuk tidak mengubah",
  config_general_app_name: "Nama platform",
  config_general_address: "Alamat",
  config_general_logo_url: "URL Logo",
  config_general_contact_email: "Email kontak",
  config_general_contact_phone: "Telepon kontak",
  students_field_provinsi: "Provinsi",
  students_field_kota: "Kota/Kabupaten",
  students_field_kecamatan: "Kecamatan",
  students_field_kode_pos: "Kode Pos",
  config_payment_server_key: "Midtrans Server Key",
  config_payment_client_key: "Midtrans Client Key",
  config_payment_env: "Lingkungan Midtrans",
  config_payment_placeholder_server: "Isi server key",
  config_payment_placeholder_client: "Isi client key",
  config_tab_general: "Umum",
  config_tab_payment: "Pembayaran",
  config_tab_features: "Fitur",
  config_tab_notifications: "Notifikasi",
  config_feature_selfreg_label: "Registrasi mandiri siswa",
  config_feature_selfreg_desc: "Izinkan siswa mendaftar langsung dari halaman publik.",
  config_feature_otp_label: "OTP wajib saat login",
  config_feature_otp_desc: "Kirimkan kode verifikasi untuk setiap percobaan login.",
  config_notif_store_label: "Notifikasi pembelian (Store Manager)",
  config_notif_store_desc: "Kirim notifikasi ke admin store saat ada pembelian baru.",
  config_notif_exam_label: "Notifikasi pembelian (Admin Exam)",
  config_notif_exam_desc: "Kirim notifikasi ke admin exam saat ada pembelian baru.",
  config_toast_general_saved: "Pengaturan umum disimpan",
  config_toast_notif_saved: "Pengaturan notifikasi disimpan",
  config_toast_payment_saved: "Pengaturan pembayaran disimpan",
  config_tab_integrations: "Integrasi Pembayaran",
  config_tab_couriers: "Integrasi Kurir",
  config_couriers_section: "Biteship Kurir",
  config_couriers_hint: "Pilih layanan kurir yang bisa ditawarkan kepada pembeli saat checkout.",
  config_couriers_summary: "{enabled} dari {total} layanan aktif",
  config_couriers_col_courier: "Kurir",
  config_couriers_col_services: "Layanan yang ditawarkan",
  config_couriers_active_suffix: "aktif",
  config_couriers_footer_hint: "Perubahan layanan berlaku setelah disimpan.",
  config_couriers_required: "Setidaknya satu layanan kurir harus tetap aktif.",
  config_group_identity: "Identitas",
  config_group_contact: "Kontak",
  config_group_address: "Alamat",
  config_group_payment: "Pembayaran",
  config_group_shipping: "Pengiriman",
  config_address_shipping_hint:
    "Alamat ini juga dipakai sebagai lokasi asal pengiriman untuk pesanan toko.",
  config_toast_integrations_saved: "Integrasi disimpan",
  sys_loading: "Memuat…",
  sys_loading_data: "Memuat data…",
  sys_error_title: "Terjadi kesalahan",
  sys_error_load: "Gagal memuat data. Coba refresh halaman.",
  sys_error_forbidden: "Akses ditolak. Hanya Super Admin yang dapat mengakses halaman ini.",
  sys_save_failed: "Gagal menyimpan",
};

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    t: (key: string) => translationMap[key] || key,
  }),
}));

const mockMutateAsync = vi.fn();
let configState = {
  data: {
    app_name: "Test App",
    app_address: "Jakarta",
    app_logo_url: "https://example.com/logo.png",
    app_contact_email: "admin@example.com",
    app_contact_phone: "+62812345678",
    app_province_id: "p1",
    app_city_id: "c1",
    app_district_id: "d1",
    app_kode_pos: "12130",
    app_latitude: "-6.2088",
    app_longitude: "106.8456",
    midtrans_server_key: "***",
    midtrans_client_key: "***",
    midtrans_env: "sandbox",
    shipping_fallback_flat_rate: "50000",
    biteship_api_key: "***",
    biteship_services: "",
    biteship_webhook_secret: "***",
    notify_on_purchase_admin_store: "false",
    notify_on_purchase_admin_exam: "false",
  },
  isLoading: false,
  isError: false,
  error: null as Error | null,
};

let updateState = {
  mutateAsync: mockMutateAsync,
  isPending: false,
};

const provincesData = [
  { id: "p1", name: "DKI Jakarta" },
  { id: "p2", name: "Jawa Barat" },
];
const citiesData = [
  { id: "c1", province_id: "p1", name: "Jakarta Selatan" },
  { id: "c2", province_id: "p1", name: "Jakarta Pusat" },
];
const districtsData = [
  { id: "d1", city_id: "c1", name: "Kebayoran Baru" },
  { id: "d2", city_id: "c1", name: "Tebet" },
];

vi.mock("@/lib/hooks/admin-config", () => ({
  useAdminSystemConfig: () => configState,
  useUpdateSystemConfig: () => updateState,
}));

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: provincesData, isLoading: false }),
  useCitiesByProvince: (provinceId?: string | null) => ({
    data: provinceId === "p1" ? citiesData : [],
    isLoading: false,
  }),
  useDistrictsByCity: (cityId?: string | null) => ({
    data: cityId === "c1" ? districtsData : [],
    isLoading: false,
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ConfigPage />
    </QueryClientProvider>
  );
}

describe("SystemConfigPage — Shipping Origin & Settings", () => {
  beforeEach(() => {
    configState = {
      data: {
        app_name: "Test App",
        app_address: "Jakarta",
        app_logo_url: "https://example.com/logo.png",
        app_contact_email: "admin@example.com",
        app_contact_phone: "+62812345678",
        app_province_id: "p1",
        app_city_id: "c1",
        app_district_id: "d1",
        app_kode_pos: "12130",
        app_latitude: "-6.2088",
        app_longitude: "106.8456",
        midtrans_server_key: "***",
        midtrans_client_key: "***",
        midtrans_env: "sandbox",
        shipping_fallback_flat_rate: "50000",
        biteship_api_key: "***",
        biteship_services: "",
        biteship_webhook_secret: "***",
        notify_on_purchase_admin_store: "false",
        notify_on_purchase_admin_exam: "false",
      },
      isLoading: false,
      isError: false,
      error: null,
    };
    updateState = {
      mutateAsync: mockMutateAsync,
      isPending: false,
    };
    mockMutateAsync.mockClear();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("loads sender coordinates and sends both only in the Couriers save payload", async () => {
    mockMutateAsync.mockResolvedValue({ success: true });
    renderPage();

    const generalPanel = screen.getByRole("tabpanel");
    expect(within(generalPanel).queryByRole("textbox", { name: "Lintang pengirim" })).not.toBeInTheDocument();
    expect(within(generalPanel).queryByRole("textbox", { name: "Bujur pengirim" })).not.toBeInTheDocument();
    fireEvent.click(within(generalPanel).getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledTimes(1));
    expect(mockMutateAsync.mock.calls[0][0]).not.toHaveProperty("app_latitude");
    expect(mockMutateAsync.mock.calls[0][0]).not.toHaveProperty("app_longitude");

    await userEvent.setup().click(screen.getByRole("tab", { name: "Integrasi Kurir" }));
    const courierPanel = screen.getByRole("tabpanel");
    const latitude = within(courierPanel).getByRole("textbox", { name: "Lintang pengirim" });
    const longitude = within(courierPanel).getByRole("textbox", { name: "Bujur pengirim" });
    expect(within(courierPanel).getByText(translationMap.config_shipping_coordinate_hint)).toBeInTheDocument();
    await waitFor(() => {
      expect(latitude).toHaveValue("-6.2088");
      expect(longitude).toHaveValue("106.8456");
    });
    fireEvent.change(latitude, { target: { value: "-6.3" } });
    fireEvent.change(longitude, { target: { value: "106.9" } });
    fireEvent.click(within(courierPanel).getByRole("button", { name: "Simpan" }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenNthCalledWith(2, expect.objectContaining({
        app_latitude: "-6.3",
        app_longitude: "106.9",
      }));
    });
  });

  it("shows the address block with its shipping-origin hint in General tab", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/asal pengiriman|shipping origin/i)).toBeInTheDocument();
    });
  });

  it("includes location fields in General tab save payload", async () => {
    mockMutateAsync.mockResolvedValue({ success: true });
    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/asal pengiriman|shipping origin/i)).toBeInTheDocument();
    });

    // Find and modify the kode_pos input
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    // Last input in general section is kode_pos
    const lastGeneralInput = inputs[inputs.length - 1];
    fireEvent.change(lastGeneralInput, { target: { value: "98765" } });

    // Get all save buttons and click the first one (General tab)
    const saveButtons = screen.getAllByRole("button", { name: /simpan|save/i });
    fireEvent.click(saveButtons[0]);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalled();
    });

    const payload = mockMutateAsync.mock.calls[0][0];
    // Verify the location fields are in the payload
    expect(payload).toHaveProperty("app_province_id");
    expect(payload).toHaveProperty("app_city_id");
    expect(payload).toHaveProperty("app_district_id");
    expect(payload).toHaveProperty("app_kode_pos");
  });

  async function openCouriers() {
    renderPage();
    await userEvent.setup().click(await screen.findByRole("tab", { name: "Integrasi Kurir" }));
    return screen.getByRole("tabpanel");
  }

  it("renders the courier section heading and checkout hint", async () => {
    const panel = await openCouriers();
    expect(within(panel).getByRole("heading", { name: "Biteship Kurir" })).toBeInTheDocument();
    expect(within(panel).getByText("Pilih layanan kurir yang bisa ditawarkan kepada pembeli saat checkout.")).toBeInTheDocument();
    expect(within(panel).getByText("dari 15 layanan aktif")).toHaveTextContent("3 dari 15 layanan aktif");
    expect(within(panel).getByText("Kurir")).toBeInTheDocument();
    expect(within(panel).getByText("Layanan yang ditawarkan")).toBeInTheDocument();
    expect(within(panel).getByText("Perubahan layanan berlaku setelah disimpan.")).toBeInTheDocument();
  });

  it("shows default counts per courier row and updates them when toggled", async () => {
    const panel = await openCouriers();
    const counts = [["JNE", "3 / 4 aktif"], ["SiCepat", "0 / 3 aktif"], ["Anteraja", "0 / 2 aktif"], ["TIKI", "0 / 4 aktif"], ["Lion Parcel", "0 / 2 aktif"]];
    for (const [name, count] of counts) {
      const row = within(panel).getByRole("img", { name }).closest("article");
      expect(row).not.toBeNull();
      expect(row).toHaveTextContent(count);
      expect(within(row!).getByText(name)).toBeInTheDocument();
    }
    const lionRow = within(panel).getByRole("img", { name: "Lion Parcel" }).closest("article")!;
    await userEvent.setup().click(within(lionRow).getByRole("switch", { name: "Lion Parcel Reg Pack" }));
    expect(lionRow).toHaveTextContent("1 / 2 aktif");
    expect(within(panel).getByText("dari 15 layanan aktif")).toHaveTextContent("4 dari 15 layanan aktif");
    const jneRow = within(panel).getByRole("img", { name: "JNE" }).closest("article")!;
    expect(within(jneRow).getByText("Reguler")).toBeInTheDocument();
    expect(within(jneRow).queryByText("JNE Reguler")).not.toBeInTheDocument();
    await userEvent.setup().click(within(jneRow).getByRole("switch", { name: "JNE Reguler" }));
    expect(jneRow).toHaveTextContent("2 / 4 aktif");
  });

  it("defines courier translations in both locales", async () => {
    const { DICT } = await vi.importActual<typeof import("@/lib/i18n")>("@/lib/i18n");
    expect(DICT.id).toMatchObject({
      config_couriers_section: "Biteship Kurir",
      config_couriers_hint: "Pilih layanan kurir yang bisa ditawarkan kepada pembeli saat checkout.",
      config_couriers_summary: "{enabled} dari {total} layanan aktif",
      config_couriers_col_courier: "Kurir",
      config_couriers_col_services: "Layanan yang ditawarkan",
      config_couriers_active_suffix: "aktif",
      config_couriers_footer_hint: "Perubahan layanan berlaku setelah disimpan.",
    });
    expect(DICT.en).toMatchObject({
      config_couriers_section: "Biteship Couriers",
      config_couriers_hint: "Choose the courier services that can be offered to buyers at checkout.",
      config_couriers_summary: "{enabled} of {total} services active",
      config_couriers_col_courier: "Courier",
      config_couriers_col_services: "Offered services",
      config_couriers_active_suffix: "active",
      config_couriers_footer_hint: "Service changes apply after saving.",
    });
  });

  it("renders courier logos, all 15 service switches and default services for empty config", async () => {
    const panel = await openCouriers();
    const names = ["JNE", "SiCepat", "Anteraja", "TIKI", "Lion Parcel"];
    const paths = ["jne.svg", "sicepat.svg", "anteraja.png", "tiki.png", "lion.svg"];
    names.forEach((name, i) => expect(within(panel).getByRole("img", { name })).toHaveAttribute("src", `/couriers/${paths[i]}`));
    const serviceNames = [
      "JNE Reguler", "JNE YES", "JNE OKE", "JNE JTR",
      "SiCepat Reguler", "SiCepat Best", "SiCepat GOKIL",
      "Anteraja Reguler", "Anteraja Same Day",
      "TIKI REG", "TIKI EKO", "TIKI ONS", "TIKI SDS",
      "Lion Parcel Reg Pack", "Lion Parcel Big Pack",
    ];
    expect(within(panel).getAllByRole("switch").map(el => el.getAttribute("aria-label"))).toEqual(serviceNames);
    for (const name of ["JNE Reguler", "JNE YES", "JNE OKE"]) {
      expect(within(panel).getByRole("switch", { name })).toHaveAttribute("aria-checked", "true");
    }
    expect(within(panel).getAllByRole("switch").filter(el => el.getAttribute("aria-checked") === "true")).toHaveLength(3);
    for (const name of names) {
      expect(within(panel).getByRole("img", { name }).className).not.toMatch(/grayscale|opacity-/);
    }
    await userEvent.setup().click(within(panel).getByRole("switch", { name: "Lion Parcel Reg Pack" }));
    expect(within(panel).getByRole("img", { name: "Lion Parcel" }).className).not.toMatch(/grayscale|opacity-/);
    expect(within(panel).queryByRole("spinbutton")).toBeNull();
    expect(within(panel).getByText("API Key Biteship")).toBeInTheDocument();
  });

  it("keeps default courier logos full colour and service chips 44 px tall", async () => {
    const panel = await openCouriers();
    expect(within(panel).getAllByRole("img").every(logo => !/opacity-|grayscale/.test(logo.className))).toBe(true);
    expect(within(panel).getByRole("switch", { name: "SiCepat Best" })).toHaveClass("h-6");
  });

  it("shows an inline warning and disables Save when all services are off", async () => {
    const panel = await openCouriers();
    for (const name of ["JNE Reguler", "JNE YES", "JNE OKE"]) {
      await userEvent.setup().click(within(panel).getByRole("switch", { name }));
    }
    expect(within(panel).getByRole("alert")).toHaveTextContent("Setidaknya satu layanan kurir harus tetap aktif.");
    expect(within(panel).getByRole("button", { name: "Simpan" })).toBeDisabled();
  });

  it("has no flat-rate field even when config contains a stale row", async () => {
    const panel = await openCouriers();
    expect(within(panel).queryByText("Tarif Fallback Pengiriman")).toBeNull();
    expect(within(panel).queryByRole("spinbutton")).toBeNull();
  });

  it("courier Save sends only shipping keys, masked secrets and services in catalog order", async () => {
    const panel = await openCouriers();
    await userEvent.setup().click(within(panel).getByRole("switch", { name: "Lion Parcel Reg Pack" }));
    await userEvent.setup().click(within(panel).getByRole("switch", { name: "JNE OKE" }));
    await userEvent.setup().click(within(panel).getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith({
      biteship_api_key: "***", biteship_webhook_secret: "***",
      app_latitude: "-6.2088", app_longitude: "106.8456",
      biteship_services: "jne:reg,jne:yes,lion:reg_pack",
    }));
  });

  it("payment tab holds Midtrans fields and saves only payment keys", async () => {
    renderPage();
    await userEvent.setup().click(await screen.findByRole("tab", { name: "Integrasi Pembayaran" }));
    const panel = screen.getByRole("tabpanel");
    expect(within(panel).getByText("Midtrans Server Key")).toBeInTheDocument();
    expect(within(panel).getByText("Midtrans Client Key")).toBeInTheDocument();
    expect(within(panel).getByText("Lingkungan Midtrans")).toBeInTheDocument();
    expect(within(panel).queryByText("API Key Biteship")).not.toBeInTheDocument();
    await userEvent.setup().click(within(panel).getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith({
      midtrans_server_key: "***", midtrans_client_key: "***", midtrans_env: "sandbox",
    }));
  });

  it.each([
    ["biteship_api_key", "new_api_key_xyz", 0],
    ["biteship_webhook_secret", "test-webhook-secret", 1],
  ])("sends edited %s from courier tab", async (key, value, index) => {
    const panel = await openCouriers();
    const inputs = panel.querySelectorAll('input[type="password"]');
    expect(inputs).toHaveLength(2);
    fireEvent.change(inputs[index], { target: { value } });
    await userEvent.setup().click(within(panel).getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith(expect.objectContaining({ [key]: value })));
  });

  it("loads a saved service set without adding defaults", async () => {
    configState.data.biteship_services = "lion:reg_pack";
    const panel = await openCouriers();
    expect(within(panel).getByRole("switch", { name: "Lion Parcel Reg Pack" })).toHaveAttribute("aria-checked", "true");
    expect(within(panel).getByRole("switch", { name: "JNE Reguler" })).toHaveAttribute("aria-checked", "false");
  });
  it("serializes saved services in catalog order", async () => {
    configState.data.biteship_services = "lion:reg_pack,jne:yes,jne:reg";
    const panel = await openCouriers();
    await userEvent.setup().click(within(panel).getByRole("button", { name: "Simpan" }));
    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith(expect.objectContaining({
      biteship_services: "jne:reg,jne:yes,lion:reg_pack",
    })));
  });

});
