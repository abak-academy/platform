import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import StudentLayout from "./layout";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

vi.mock("next/script", () => ({
  default: () => null,
}));

vi.mock("@/components/shell/AppShell", () => ({
  AppShell: ({ role, children }: { role: string; children: React.ReactNode }) => (
    <div data-testid="shell" data-role={role}>
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

let profileState = {
  data: undefined as
    | { auth_provider: "google" | "password"; school_id?: string; grade?: number }
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
    replace.mockClear();
    PasswordTemporaryModal.mockClear();
    authStore = { token: null, user: null };
    roleState = { role: undefined, hydrated: true, meIsError: false };
    meState = { data: undefined };
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
});
