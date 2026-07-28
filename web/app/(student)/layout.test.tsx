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

describe("StudentLayout", () => {
  beforeEach(() => {
    replace.mockClear();
    authStore = { token: null, user: null };
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
    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);
    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(getByTestId("shell")).toHaveAttribute("data-role", "student");
  });

  it("redirects to /admin when the role is an admin role", async () => {
    authStore = { token: "t", user: { role: "admin_store" } };
    render(<StudentLayout>protected</StudentLayout>);
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/admin"));
  });

  it("redirects an incomplete Google student to /complete-profile", async () => {
    authStore = { token: "t", user: { role: "student" } };
    profileState.data = { auth_provider: "google" };

    const { queryByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/complete-profile"));
    expect(queryByTestId("shell")).not.toBeInTheDocument();
  });

  it("renders the shell for a complete Google student", async () => {
    authStore = { token: "t", user: { role: "student" } };
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
    profileState.data = { auth_provider: "password" };

    const { getByTestId } = render(<StudentLayout>protected</StudentLayout>);

    await waitFor(() => expect(getByTestId("shell")).toBeInTheDocument());
    expect(replace).not.toHaveBeenCalledWith("/complete-profile");
  });

  it("does not trust cached profile data while DB truth is refetching", async () => {
    authStore = { token: "t", user: { role: "student" } };
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
});
