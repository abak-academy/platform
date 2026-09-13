import * as React from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CompleteProfilePage from "./page";

const replace = vi.fn();
const mutateAsync = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

let authStore = {
  token: "token" as string | null,
  user: { name: "Google Student" } as { name?: string } | null,
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (state: typeof authStore) => unknown) => selector(authStore),
}));

type ProfileData = {
  auth_provider: "google" | "password";
  school_id?: string;
  grade?: number;
};

let profileState: { data: ProfileData; isLoading: boolean } = {
  data: { auth_provider: "google" },
  isLoading: false,
};

vi.mock("@/lib/hooks/students", () => ({
  studentsKeys: { profile: () => ["students", "profile"] },
  useProfile: () => profileState,
  useSchools: () => ({
    data: [{ id: "school-1", name: "School One" }],
    isLoading: false,
  }),
  useUpdateProfile: () => ({ mutateAsync }),
}));

vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    value,
    onValueChange,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => (
    <select value={value} onChange={(event) => onValueChange(event.target.value)}>
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => (
    <option value={value}>{children}</option>
  ),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe("CompleteProfilePage", () => {
  beforeEach(() => {
    replace.mockReset();
    mutateAsync.mockReset();
    mutateAsync.mockResolvedValue({});
    authStore = { token: "token", user: { name: "Google Student" } };
    profileState = { data: { auth_provider: "google" }, isLoading: false };
  });

  it("renders school and grade fields for an incomplete Google student", () => {
    renderPage();

    expect(screen.getByLabelText("Nama")).toHaveValue("Google Student");
    expect(screen.getAllByRole("combobox")).toHaveLength(2);
  });

  it("waits for profile invalidation before reopening the student app", async () => {
    const invalidation = deferred<void>();
    const queryClient = new QueryClient();
    vi.spyOn(queryClient, "invalidateQueries").mockReturnValue(invalidation.promise);
    renderPage(queryClient);

    const [school, grade] = screen.getAllByRole("combobox");
    fireEvent.change(school, { target: { value: "school-1" } });
    fireEvent.change(grade, { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Lanjutkan" }));

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        name: "Google Student",
        school_id: "school-1",
        grade: 12,
      }),
    );
    expect(replace).not.toHaveBeenCalledWith("/");

    invalidation.resolve();
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
  });

  it.each([
    ["complete Google", { auth_provider: "google" as const, school_id: "school-1", grade: 12 }],
    ["password", { auth_provider: "password" as const }],
  ])("redirects a %s profile away from onboarding", async (_name, data) => {
    profileState = { data, isLoading: false };

    renderPage();

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
  });

  it("redirects to login without a token", async () => {
    authStore = { token: null, user: null };

    renderPage();

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
  });

  it("swaps the school select for a free-text input when the unlisted option is chosen", () => {
    renderPage();

    const [school] = screen.getAllByRole("combobox");
    fireEvent.change(school, { target: { value: "_unlisted_" } });

    // Select should be gone (or hidden), replaced by a free-text input.
    expect(screen.queryByRole("combobox", { name: /sekolah/i })).toBeNull();
    const freeText = screen.getByLabelText("Tulis nama sekolah Anda");
    expect(freeText).toBeInTheDocument();
    expect(freeText.tagName).toBe("INPUT");
  });

  it("submits unlisted_school_name (and never school_id) when the unlisted option is used", async () => {
    renderPage();

    const [school, grade] = screen.getAllByRole("combobox");
    fireEvent.change(school, { target: { value: "_unlisted_" } });
    fireEvent.change(grade, { target: { value: "11" } });
    const freeText = screen.getByLabelText("Tulis nama sekolah Anda");
    fireEvent.change(freeText, { target: { value: "SMA Maju Bersama" } });
    fireEvent.click(screen.getByRole("button", { name: "Lanjutkan" }));

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        name: "Google Student",
        unlisted_school_name: "SMA Maju Bersama",
        grade: 11,
      }),
    );
    // The "real school" path must not also be sent on the unlisted path.
    const callArg = mutateAsync.mock.calls[0][0] as Record<string, unknown>;
    expect(callArg).not.toHaveProperty("school_id");
  });
});

function renderPage(queryClient = new QueryClient()) {
  return render(
    <QueryClientProvider client={queryClient}>
      <CompleteProfilePage />
    </QueryClientProvider>,
  );
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
