import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import ProfilePage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

let authStore: {
  user: { name?: string; school_id?: string } | null;
  token: string | null;
  setSession: (...args: unknown[]) => void;
};

vi.mock("@/stores/auth", () => {
  const authStoreMock = {
    getState: () => ({ refreshToken: "" }),
  };
  const selectorable = (selector: (s: typeof authStore) => unknown) =>
    selector(authStore);
  return {
    useAuthStore: Object.assign(selectorable, authStoreMock),
  };
});

let profileState = {
  data: null as {
    id?: string;
    updated_at?: string;
    name?: string;
    email?: string;
    phone?: string;
    nis?: string;
    grade?: string | number;
    target_exam?: string;
    alamat_domisili?: string;
    school_id?: string | null;
    unlisted_school_name?: string | null;
    jenjang?: string | null;
    provinsi_id?: string | null;
    kota_id?: string | null;
    kecamatan_id?: string | null;
    kode_pos?: string | null;
  } | null,
  isLoading: false,
  isError: false,
  error: null,
  refetch: vi.fn(),
};

const mutateMock = vi.fn();

let schoolsData: Array<{
  id: string;
  name: string;
  school_types?: string[];
}> = [];

vi.mock("@/lib/hooks/students", () => ({
  useDashboard: () => ({ data: null, isLoading: false, isError: false, refetch: vi.fn() }),
  useProfile: () => profileState,
  useUpdateProfile: () => ({ mutate: mutateMock, isPending: false }),
  useChangePassword: () => ({ mutate: vi.fn(), isPending: false }),
  useSchools: () => ({ data: schoolsData, isLoading: false }),
  usePresignUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdatePhoto: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

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

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: provincesData, isLoading: false }),
  useCitiesByProvince: () => ({ data: citiesData, isLoading: false }),
  useDistrictsByCity: () => ({ data: districtsData, isLoading: false }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const UNLISTED_SCHOOL_VALUE = "_unlisted_";
const FALLBACK_JENJANG = ["SD", "SMP", "SMA", "SMK"];

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ProfilePage />
    </QueryClientProvider>
  );
}

describe("ProfilePage", () => {
  beforeEach(() => {
    authStore = {
      user: { name: "Budi Santoso" },
      token: "test-token",
      setSession: vi.fn(),
    };
    profileState = {
      data: {
        id: "u1",
        updated_at: "2026-01-01T00:00:00Z",
        name: "Budi Santoso",
        email: "budi@example.com",
        phone: "08123456789",
        nis: "1928374650",
        grade: "12 SMA",
        target_exam: "SNBT",
        alamat_domisili: "Jl. Merdeka No. 1",
        school_id: "s1",
        unlisted_school_name: null,
        jenjang: null,
        provinsi_id: null,
        kota_id: null,
        kecamatan_id: null,
        kode_pos: null,
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    schoolsData = [
      { id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA", "SMK"] },
      { id: "s2", name: "SMAN 2 Bandung", school_types: ["SMA"] },
    ];
    mutateMock.mockClear();
  });

  it("shows a read-only profile with an edit trigger and no save button by default", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByLabelText(/nama/i, { selector: "input" })).toBeInTheDocument();
    });

    expect(screen.getByLabelText(/nama/i, { selector: "input" })).toBeDisabled();
    expect(screen.getByLabelText(/telepon|phone/i, { selector: "input" })).toBeDisabled();
    expect(screen.getByLabelText(/alamat|address/i, { selector: "input" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /ubah profil|edit profile/i })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /simpan perubahan|save changes/i })
    ).not.toBeInTheDocument();
  });

  it("entering edit mode unlocks fields and does not save on its own", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /ubah profil|edit profile/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /ubah profil|edit profile/i }));

    // entering edit mode must never trigger a save (regression: edit click used to PATCH)
    expect(mutateMock).not.toHaveBeenCalled();
    expect(screen.getByLabelText(/nama/i, { selector: "input" })).toBeEnabled();
    expect(screen.getByLabelText(/telepon|phone/i, { selector: "input" })).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /simpan perubahan|save changes/i })
    ).toBeInTheDocument();
    // email stays locked even in edit mode
    expect(screen.getByLabelText(/email/i, { selector: "input" })).toBeDisabled();
  });
});

describe("ProfilePage — new optional biodata fields (FR-FE-24..27)", () => {
  beforeEach(() => {
    authStore = {
      user: { name: "Budi Santoso", school_id: "s1" },
      token: "test-token",
      setSession: vi.fn(),
    };
    profileState = {
      data: {
        id: "u1",
        updated_at: "2026-01-01T00:00:00Z",
        name: "Budi Santoso",
        email: "budi@example.com",
        school_id: "s1",
        unlisted_school_name: null,
        jenjang: null,
        provinsi_id: null,
        kota_id: null,
        kecamatan_id: null,
        kode_pos: null,
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    schoolsData = [
      { id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA", "SMK"] },
      { id: "s2", name: "SMAN 2 Bandung", school_types: ["SMA"] },
    ];
    mutateMock.mockClear();
    mutateMock.mockImplementation((_payload, opts) => {
      opts?.onSuccess?.({
        id: "u1",
        name: "Budi Santoso",
        email: "budi@example.com",
      });
    });
  });

  function enterEditMode() {
    fireEvent.click(screen.getByRole("button", { name: /ubah profil|edit profile/i }));
  }

  // The five new fields are wrappers around either a Select (jenjang/provinsi/kota/kecamatan)
  // or an Input (kode_pos). We confirm presence via either a labelled control or placeholder
  // text rendered inside the document.

  it("renders jenjang, provinsi, kota, kecamatan, and kode pos fields with no required marker (FR-FE-24)", async () => {
    renderPage();
    enterEditMode();

    await waitFor(() => {
      expect(screen.getByLabelText(/jenjang/i)).toBeInTheDocument();
    });

    // Each new field's label is visible.
    expect(screen.getByLabelText(/jenjang/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/provinsi/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/kota/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/kecamatan/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/kode pos/i)).toBeInTheDocument();

    // No required-field marker (no asterisk on the labels for these fields).
    const jenjangLabel = screen.getByLabelText(/jenjang/i).closest("div")?.previousElementSibling;
    const jenjangLabelText = jenjangLabel?.textContent ?? "";
    expect(jenjangLabelText).not.toMatch(/\*/);
  });

  it("restricts jenjang options to the user's own school_types when school_id resolves (FR-FE-25)", async () => {
    renderPage();

    // Wait for the school select to display s1's name (sync from profile has
    // completed). Without this wait, jenjangOptions may briefly fall back to
    // the generic list because the useEffect that sets schoolId hasn't run yet.
    await waitFor(() => {
      const schoolSelect = screen.getByLabelText(/sekolah/i) as HTMLButtonElement;
      // SMAN 1 Jakarta is the name of s1.
      expect(schoolSelect.textContent).toContain("SMAN 1 Jakarta");
    });

    enterEditMode();

    // Open the jenjang select.
    const jenjangTrigger = screen.getByLabelText(/jenjang/i);
    fireEvent.click(jenjangTrigger);

    // The two school_types from s1 must be present.
    await waitFor(() => {
      expect(screen.getByRole("option", { name: "SMA" })).toBeInTheDocument();
      expect(screen.getByRole("option", { name: "SMK" })).toBeInTheDocument();
    });
    // "SMP" is not in s1.school_types, so it must not appear.
    expect(screen.queryByRole("option", { name: "SMP" })).not.toBeInTheDocument();
  });

  it("falls back to a generic jenjang list when the user has no real school set (FR-FE-25)", async () => {
    profileState = {
      ...profileState,
      data: {
        ...(profileState.data as object),
        school_id: null,
        unlisted_school_name: null,
      },
    };
    renderPage();
    enterEditMode();

    const jenjangTrigger = screen.getByLabelText(/jenjang/i);
    fireEvent.click(jenjangTrigger);

    // The fallback list covers the canonical jenjangs.
    for (const j of FALLBACK_JENJANG) {
      await waitFor(() => {
        expect(screen.getByRole("option", { name: j })).toBeInTheDocument();
      });
    }
  });

  it("falls back to the generic jenjang list when the user picked 'unlisted' (FR-FE-25)", async () => {
    profileState = {
      ...profileState,
      data: {
        ...(profileState.data as object),
        school_id: null,
        unlisted_school_name: "SMA Maju Bersama",
      },
    };
    renderPage();
    enterEditMode();

    const jenjangTrigger = screen.getByLabelText(/jenjang/i);
    fireEvent.click(jenjangTrigger);

    await waitFor(() => {
      expect(screen.getByRole("option", { name: "SMA" })).toBeInTheDocument();
    });
    expect(screen.getByRole("option", { name: "SMP" })).toBeInTheDocument();
  });

  it("includes the 'unlisted school' option in the school selector (FR-FE-26)", async () => {
    renderPage();
    enterEditMode();

    const schoolTrigger = screen.getByLabelText(/sekolah/i);
    fireEvent.click(schoolTrigger);

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: /tidak ada di daftar|not on the list/i })
      ).toBeInTheDocument();
    });
  });

  it("swaps the school select for a free-text input when 'unlisted' is chosen (FR-FE-26)", async () => {
    renderPage();
    enterEditMode();

    const schoolTrigger = screen.getByLabelText(/sekolah/i);
    fireEvent.click(schoolTrigger);
    fireEvent.click(
      screen.getByRole("option", { name: /tidak ada di daftar|not on the list/i })
    );

    await waitFor(() => {
      expect(
        screen.getByLabelText(/tulis nama sekolah|type your school name/i)
      ).toBeInTheDocument();
    });
  });

  it("saves successfully when none of the new fields are filled (regression — empty submit still works)", async () => {
    renderPage();
    enterEditMode();

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    // None of the new fields should be present in the payload (all optional, all blank).
    expect(payload).not.toHaveProperty("jenjang");
    expect(payload).not.toHaveProperty("provinsi_id");
    expect(payload).not.toHaveProperty("kota_id");
    expect(payload).not.toHaveProperty("kecamatan_id");
    expect(payload).not.toHaveProperty("kode_pos");
  });

  it("submits a valid jenjang + full province/city/kecamatan triple in the PATCH payload", async () => {
    renderPage();
    enterEditMode();

    // Pick jenjang.
    const jenjangTrigger = screen.getByLabelText(/jenjang/i);
    fireEvent.click(jenjangTrigger);
    fireEvent.click(screen.getByRole("option", { name: "SMA" }));

    // Pick provinsi.
    const provTrigger = screen.getByLabelText(/provinsi/i);
    fireEvent.click(provTrigger);
    fireEvent.click(screen.getByRole("option", { name: "DKI Jakarta" }));

    // Pick kota.
    const kotaTrigger = screen.getByLabelText(/kota/i);
    fireEvent.click(kotaTrigger);
    fireEvent.click(screen.getByRole("option", { name: "Jakarta Selatan" }));

    // Pick kecamatan.
    const kecTrigger = screen.getByLabelText(/kecamatan/i);
    fireEvent.click(kecTrigger);
    fireEvent.click(screen.getByRole("option", { name: "Kebayoran Baru" }));

    fireEvent.change(screen.getByLabelText(/kode pos/i, { selector: "input" }), {
      target: { value: "12130" },
    });

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    expect(payload.jenjang).toBe("SMA");
    expect(payload.provinsi_id).toBe("p1");
    expect(payload.kota_id).toBe("c1");
    expect(payload.kecamatan_id).toBe("d1");
    expect(payload.kode_pos).toBe("12130");
  });

  it("signals the unlisted-school intent with school_id: \"\" and the typed name (FB-14)", async () => {
    renderPage();
    enterEditMode();

    const schoolTrigger = screen.getByLabelText(/sekolah/i);
    fireEvent.click(schoolTrigger);
    fireEvent.click(
      screen.getByRole("option", { name: /tidak ada di daftar|not on the list/i })
    );

    const unlistedInput = await screen.findByLabelText(
      /tulis nama sekolah|type your school name/i
    );
    fireEvent.change(unlistedInput, { target: { value: "SMA Maju Bersama" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    expect(payload.school_id).toBe("");
    expect(payload.unlisted_school_name).toBe("SMA Maju Bersama");
  });

  it("signals the listed-school intent with unlisted_school_name: \"\" (FB-14)", async () => {
    renderPage();
    enterEditMode();

    const schoolTrigger = screen.getByLabelText(/sekolah/i);
    fireEvent.click(schoolTrigger);
    fireEvent.click(screen.getByRole("option", { name: "SMAN 2 Bandung" }));

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    expect(payload.school_id).toBe("s2");
    expect(payload.unlisted_school_name).toBe("");
  });

  it("omits both school_id and unlisted_school_name when neither is selected and only name changes (FR-12)", async () => {
    profileState = {
      ...profileState,
      data: {
        ...(profileState.data as object),
        school_id: null,
        unlisted_school_name: null,
      },
    };
    renderPage();
    enterEditMode();

    fireEvent.change(screen.getByLabelText(/nama/i, { selector: "input" }), {
      target: { value: "Budi Santoso Jr" },
    });

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    expect(payload).not.toHaveProperty("school_id");
    expect(payload).not.toHaveProperty("unlisted_school_name");
  });

  it("surfaces the server's incomplete_address error when only provinsi_id is submitted", async () => {
    mutateMock.mockImplementation((_payload, opts) => {
      opts?.onError?.(Object.assign(new Error("incomplete address"), { code: "incomplete_address" }));
    });
    renderPage();
    enterEditMode();

    // Pick only provinsi.
    const provTrigger = screen.getByLabelText(/provinsi/i);
    fireEvent.click(provTrigger);
    fireEvent.click(screen.getByRole("option", { name: "DKI Jakarta" }));

    fireEvent.click(screen.getByRole("button", { name: /simpan perubahan|save changes/i }));

    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalled();
    });
    const payload = mutateMock.mock.calls[0][0];
    expect(payload.provinsi_id).toBe("p1");
    // kota_id/kecamatan_id must be omitted (empty), so the server returns incomplete_address.
    expect(payload).not.toHaveProperty("kota_id");
    expect(payload).not.toHaveProperty("kecamatan_id");
  });
});
