import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import StudentLayout from "./layout";
import { useCartStore } from "@/stores/cart";

const replace = vi.fn();

let pathname = "/orders";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => pathname,
}));

const useGuestCartMerge = vi.fn();
vi.mock("@/lib/guest-cart-merge", () => ({
  useGuestCartMerge: (enabled: boolean) => useGuestCartMerge(enabled),
}));

vi.mock("@/components/shell/PublicShell", () => ({
  PublicShell: ({ children, guestOnly }: { children: React.ReactNode; guestOnly?: boolean }) => (
    <div data-testid="public-shell" data-guest-only={guestOnly ? "" : undefined}>{children}</div>
  ),
}));

vi.mock("next/script", () => ({
  default: () => null,
}));

vi.mock("@/components/shell/AppShell", () => ({
  AppShell: ({
    role,
    counts,
    subtitle,
    children,
  }: {
    role: string;
    counts?: Record<string, number>;
    subtitle?: string;
    children: React.ReactNode;
  }) => (
    <div data-testid="shell" data-role={role} data-counts={JSON.stringify(counts)} data-subtitle={subtitle}>
      {children}
    </div>
  ),
}));

let authStore = {
  token: null as string | null,
  user: null as { role?: string } | null,
};

let roleState = {
  role: undefined as string | undefined,
  hydrated: true,
  meIsError: false,
};

let regsState = { data: [] as unknown[] };
let ordersState = { data: [] as unknown[] };
let schoolState = { data: undefined as { name: string } | undefined };

vi.mock("@/lib/hooks/exam", () => ({ useRegistrations: () => regsState }));
const signedInHooks = vi.hoisted(() => ({ orders: vi.fn() }));
vi.mock("@/lib/hooks/orders", () => ({ useOrders: () => (signedInHooks.orders(), ordersState) }));
vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => (k === "grade" ? "Kelas" : k) }) }));

let profileState = {
  data: undefined as
    | { auth_provider: "google" | "password"; school_id?: string; grade?: number; unlisted_school_name?: string | null }
    | undefined,
  isLoading: false,
  isFetching: false,
  isError: false,
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

vi.mock("@/lib/hooks/students", () => ({
  useProfile: () => profileState,
  useSchoolById: () => schoolState,
}));

vi.mock("@/lib/hooks/use-capability", () => ({
  useResolvedRole: () => roleState,
}));

let meState = {
  data: undefined as { password_temporary?: boolean } | undefined,
};

vi.mock("@/lib/hooks/auth", () => ({
  useMe: () => meState,
}));

const PasswordTemporaryModal = vi.fn(
  ({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) =>
    open ? (
      <button data-testid="pw-notice" onClick={() => onOpenChange(false)}>
        notice
      </button>
    ) : null
);

vi.mock("@/components/auth/PasswordTemporaryModal", () => ({
  PasswordTemporaryModal: (props: { open: boolean; onOpenChange: (v: boolean) => void }) =>
    PasswordTemporaryModal(props),
}));

describe("StudentLayout", () => {
  beforeEach(() => {
    signedInHooks.orders.mockClear();
    replace.mockClear();
    useGuestCartMerge.mockClear();
    pathname = "/orders";
    PasswordTemporaryModal.mockClear();
    authStore = { token: null, user: null };
    roleState = { role: undefined, hydrated: true, meIsError: false };
    meState = { data: undefined };
    regsState = { data: [] };
    ordersState = { data: [] };
    schoolState = { data: undefined };
    useCartStore.setState({ count: 0 });
    profileState = {
      data: undefined,
      isLoading: false,
      isFetching: false,
      isError: false,
    };
  });

  it("redirects to /login when there is no token", async () => {
    render(<StudentLayout>protected</StudentLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
  });

  it("renders the shell for student role", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);
    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(getByTestId("shell")).toHaveAttribute("data-role", "student");
  });

  it("redirects to /admin when the role is an admin role", async () => {
    authStore = { token: "t", user: { role: "admin_store" } };
    roleState = { role: "admin_store", hydrated: true, meIsError: false };
    render(<StudentLayout>protected</StudentLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin"));
  });

  it("redirects an affiliator to the affiliate portal without rendering the student shell", async () => {
    authStore = { token: "t", user: { role: "affiliator" } };
    roleState = { role: "affiliator", hydrated: true, meIsError: false };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/affiliate"));
    expect(queryByTestId("shell")).not.toBeInTheDocument();
  });

  // The reported bug: a persisted session held a valid super_admin token but no
  // user, so the store's role was undefined and the admin redirect never fired.
  // The role now comes from /auth/me, so the redirect happens anyway.
  it("redirects to /admin on a server-resolved admin role the store never had", async () => {
    authStore = { token: "t", user: null };
    roleState = { role: "super_admin", hydrated: true, meIsError: false };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin"));
    expect(queryByTestId("shell")).not.toBeInTheDocument();
  });

  it("holds the shell back while the role is still resolving", async () => {
    authStore = { token: "t", user: null };
    roleState = { role: undefined, hydrated: true, meIsError: false };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(queryByTestId("shell")).not.toBeInTheDocument());
    expect(replace).not.toHaveBeenCalledWith("/admin");
  });

  it("redirects to /login when the server will not confirm the identity", async () => {
    authStore = { token: "t", user: null };
    roleState = { role: undefined, hydrated: true, meIsError: true };

    render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
  });

  it("redirects an incomplete Google student to /complete-profile", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    profileState.data = { auth_provider: "google" };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/complete-profile"));
    expect(queryByTestId("shell")).not.toBeInTheDocument();
  });

  it("renders the shell for a complete Google student", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    profileState.data = {
      auth_provider: "google",
      school_id: "school-db",
      grade: 12,
    };

    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(replace).not.toHaveBeenCalledWith("/complete-profile");
  });

  it("renders the shell for an incomplete password student", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    profileState.data = { auth_provider: "password" };

    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(replace).not.toHaveBeenCalledWith("/complete-profile");
  });

  it("does not trust cached profile data while DB truth is refetching", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    profileState = {
      data: { auth_provider: "google" },
      isLoading: false,
      isFetching: true,
      isError: false,
    };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(queryByTestId("shell")).not.toBeInTheDocument());
    expect(replace).not.toHaveBeenCalledWith("/complete-profile");
  });

  it("keeps a password student's page mounted while the profile refetches", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    profileState = {
      data: { auth_provider: "password" },
      isLoading: false,
      isFetching: true,
      isError: false,
    };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    // Nothing gates a password student on completeness, so a background refetch
    // has no reason to unmount them — and unmounting cost them whatever they had
    // typed on the page.
    await waitFor(() => expect(queryByTestId("shell")).toBeInTheDocument());
  });

  it("shows the change-password notice for a flagged student", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    meState = { data: { password_temporary: true } };

    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(getByTestId("pw-notice")).toBeInTheDocument());
  });

  it("hides the notice after it is dismissed, but keeps the shell", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    meState = { data: { password_temporary: true } };

    const { getByTestId, queryByTestId } = render(
      <StudentLayout>protected</StudentLayout>
    );

    await waitFor(() => expect(getByTestId("pw-notice")).toBeInTheDocument());
    getByTestId("pw-notice").click();

    await waitFor(() =>
      expect(queryByTestId("pw-notice")).not.toBeInTheDocument()
    );
    expect(getByTestId("shell")).toBeInTheDocument();
  });

  it("does not show the notice when the server clears the flag", async () => {
    authStore = { token: "t", user: { role: "student" } };
    roleState = { role: "student", hydrated: true, meIsError: false };
    meState = { data: { password_temporary: false } };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(queryByTestId("shell")).toBeInTheDocument());
    expect(queryByTestId("pw-notice")).not.toBeInTheDocument();
  });

  describe("shell counts and subtitle", () => {
    const reg = (status: string, over: Record<string, unknown> = {}) => ({
      status,
      max_attempts: 1,
      attempts_used: status === "submitted" ? 1 : 0,
      requires_checkin: false,
      scheduled_at: null,
      scheduled_end_at: null,
      check_in_window_minutes: null,
      ...over,
    });
    const mountStudent = async () => {
      authStore = { token: "t", user: { role: "student" } };
      roleState = { role: "student", hydrated: true, meIsError: false };
      const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);
      await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
      return getByTestId("shell");
    };

    it("passes kompetisi, tagihan and cart counts", async () => {
      regsState = { data: [reg("registered"), reg("in_progress"), reg("submitted"), reg("registered")] };
      ordersState = { data: [{ status: "payment_pending" }, { status: "paid" }, { status: "payment_pending" }] };
      useCartStore.setState({ count: 4 });
      const shell = await mountStudent();
      expect(JSON.parse(shell.dataset.counts!)).toEqual({ kompetisi: 3, tagihan: 2, cart: 4 });
    });

    it("passes zeros when nothing is pending", async () => {
      regsState = { data: [reg("submitted")] };
      ordersState = { data: [{ status: "paid" }] };
      const shell = await mountStudent();
      expect(JSON.parse(shell.dataset.counts!)).toEqual({ kompetisi: 0, tagihan: 0, cart: 0 });
    });

    it("builds the subtitle from grade and school, dropping an empty part", async () => {
      profileState.data = { auth_provider: "password", grade: 11, school_id: "s1" };
      schoolState = { data: { name: "SMAN 1" } };
      expect((await mountStudent()).dataset.subtitle).toBe("Kelas 11 · SMAN 1");
    });

    it("uses the unlisted school name and omits a missing grade", async () => {
      profileState.data = { auth_provider: "password", unlisted_school_name: "SMA Swasta" };
      expect((await mountStudent()).dataset.subtitle).toBe("SMA Swasta");
    });

    it("leaves the subtitle empty when the profile has neither", async () => {
      profileState.data = { auth_provider: "password" };
      expect((await mountStudent()).dataset.subtitle).toBe("");
    });
  });

  describe("guest paths", () => {
    it("renders the public shell for a hydrated guest on / without redirecting", async () => {
      pathname = "/";
      const { getByTestId, queryByText } = render(<StudentLayout>landing</StudentLayout>);
      expect(getByTestId("public-shell")).toHaveTextContent("landing");
      expect(queryByText("Memuat…")).not.toBeInTheDocument();
      expect(replace).not.toHaveBeenCalled();
      // the signed-in layout's endpoints 401 for a guest, and a 401 hard-navigates to /login
      expect(signedInHooks.orders).not.toHaveBeenCalled();
      expect(useGuestCartMerge).not.toHaveBeenCalled();
    });

    it("renders the public shell for a hydrated guest on /cart", () => {
      pathname = "/cart";
      const { getByTestId } = render(<StudentLayout>cart</StudentLayout>);
      expect(getByTestId("public-shell")).toBeInTheDocument();
      expect(getByTestId("public-shell")).toHaveAttribute("data-guest-only");
      expect(replace).not.toHaveBeenCalled();
    });

    it("flags the guest landing shell as guest-only", () => {
      pathname = "/";
      const { getByTestId } = render(<StudentLayout>landing</StudentLayout>);
      expect(getByTestId("public-shell")).toHaveAttribute("data-guest-only");
    });

    it("still redirects a guest on any other path", async () => {
      pathname = "/orders";
      const { queryByTestId } = render(<StudentLayout>x</StudentLayout>);
      await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
      expect(queryByTestId("public-shell")).not.toBeInTheDocument();
    });

    it("prerenders the landing on / before hydration, not Memuat", () => {
      pathname = "/";
      roleState = { role: undefined, hydrated: false, meIsError: false };
      const { getByTestId, queryByText } = render(<StudentLayout>landing</StudentLayout>);
      expect(getByTestId("public-shell")).toBeInTheDocument();
      expect(queryByText("Memuat…")).not.toBeInTheDocument();
    });

    it("keeps Memuat on /cart before hydration", () => {
      pathname = "/cart";
      roleState = { role: undefined, hydrated: false, meIsError: false };
      const { getByText, queryByTestId } = render(<StudentLayout>cart</StudentLayout>);
      expect(getByText("Memuat…")).toBeInTheDocument();
      expect(queryByTestId("public-shell")).not.toBeInTheDocument();
    });

    it("gives a student the app shell on /", async () => {
      pathname = "/";
      authStore = { token: "t", user: { role: "student" } };
      roleState = { role: "student", hydrated: true, meIsError: false };
      const { getByTestId, queryByTestId } = render(<StudentLayout>dash</StudentLayout>);
      await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
      expect(queryByTestId("public-shell")).not.toBeInTheDocument();
    });

    it("redirects an admin on / to /admin", async () => {
      pathname = "/";
      authStore = { token: "t", user: { role: "super_admin" } };
      roleState = { role: "super_admin", hydrated: true, meIsError: false };
      const { queryByTestId } = render(<StudentLayout>x</StudentLayout>);
      await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin"));
      expect(queryByTestId("public-shell")).not.toBeInTheDocument();
    });
  });

  describe("guest cart merge", () => {
    const lastEnabled = () => useGuestCartMerge.mock.calls.at(-1)?.[0];

    it("is enabled for a ready student", () => {
      authStore = { token: "t", user: { role: "student" } };
      roleState = { role: "student", hydrated: true, meIsError: false };
      render(<StudentLayout>x</StudentLayout>);
      expect(lastEnabled()).toBe(true);
    });

    it("is disabled while the profile is pending", () => {
      authStore = { token: "t", user: { role: "student" } };
      roleState = { role: "student", hydrated: true, meIsError: false };
      profileState = { data: undefined, isLoading: true, isFetching: true, isError: false };
      render(<StudentLayout>x</StudentLayout>);
      expect(lastEnabled()).toBe(false);
    });

    it("is disabled for a guest and for an admin", () => {
      render(<StudentLayout>x</StudentLayout>);
      expect(lastEnabled()).toBe(false);
      useGuestCartMerge.mockClear();
      authStore = { token: "t", user: { role: "super_admin" } };
      roleState = { role: "super_admin", hydrated: true, meIsError: false };
      render(<StudentLayout>x</StudentLayout>);
      expect(lastEnabled()).toBe(false);
    });
  });
});
