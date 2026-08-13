import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import AdminProfilePage from "./page";
import type { User } from "@/lib/types";

let meState: {
  data: User | null;
  isLoading: boolean;
} = {
  data: null,
  isLoading: false,
};

vi.mock("@/lib/hooks/auth", () => ({
  useMe: () => meState,
}));

const presignMutateAsync = vi.fn();
const updatePhotoMutateAsync = vi.fn();
const callOrder: string[] = [];

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({
    mutateAsync: (...args: unknown[]) => {
      callOrder.push("presign");
      return presignMutateAsync(...args);
    },
  }),
  useChangePassword: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/hooks/profile", () => ({
  useUpdateOwnPhoto: () => ({
    mutateAsync: (...args: unknown[]) => {
      callOrder.push("updatePhoto");
      return updatePhotoMutateAsync(...args);
    },
  }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: (...a: unknown[]) => toastError(...a) },
}));

function baseUser(): User {
  return {
    id: "u1",
    name: "Budi Santoso",
    email: "budi@abak.id",
    username: "budi",
    role: "admin_store",
    status: "active",
    auth_provider: "password",
    photo_url: "avatars/u1/old.png",
  };
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AdminProfilePage />
    </QueryClientProvider>,
  );
}

describe("AdminProfilePage", () => {
  beforeEach(() => {
    meState = { data: baseUser(), isLoading: false };
    presignMutateAsync.mockReset();
    updatePhotoMutateAsync.mockReset();
    toastSuccess.mockReset();
    toastError.mockReset();
    callOrder.length = 0;
    vi.stubGlobal("fetch", vi.fn());
  });

  it("renders the identity card with name, role label and email", () => {
    renderPage();

    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText("Store Manager")).toBeInTheDocument();
    expect(screen.getByText("budi@abak.id")).toBeInTheDocument();
  });

  it("reports the sign-in method as password, naming the Google origin for a migrated account", () => {
    renderPage();
    expect(screen.getByText("Kata sandi")).toBeInTheDocument();

    cleanup();
    meState = { data: { ...baseUser(), auth_provider: "google" }, isLoading: false };
    renderPage();

    // Google sign-in is student-only, so an admin's usable method is the password
    // even when the account was originally created through Google.
    expect(
      screen.getByText("Kata sandi (akun dibuat via Google)"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Google")).not.toBeInTheDocument();
  });

  it("keeps ChangePasswordForm collapsed until it is asked for", () => {
    renderPage();

    expect(screen.queryByLabelText("Kata sandi lama")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /ubah kata sandi/i })
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("renders ChangePasswordForm once the change-password toggle is pressed", () => {
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: /ubah kata sandi/i }));

    expect(screen.getByLabelText("Kata sandi lama")).toBeInTheDocument();
    expect(screen.getByLabelText("Kata sandi baru")).toBeInTheDocument();
    expect(screen.getByLabelText("Konfirmasi kata sandi baru")).toBeInTheDocument();
  });

  it("collapses the form again when the toggle is pressed a second time", () => {
    renderPage();

    const toggle = screen.getByRole("button", { name: /ubah kata sandi/i });
    fireEvent.click(toggle);
    expect(screen.getByLabelText("Kata sandi lama")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /batal/i }));

    expect(screen.queryByLabelText("Kata sandi lama")).not.toBeInTheDocument();
  });

  it("uploads a photo via presign, PUT, then the /auth/photo mutation, in that order", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "https://upload.example/put",
      method: "PUT",
      key: "avatars/u1/new.png",
    });
    (global.fetch as ReturnType<typeof vi.fn>).mockImplementation((...args: unknown[]) => {
      callOrder.push("put");
      return Promise.resolve({ ok: true });
    });
    updatePhotoMutateAsync.mockResolvedValueOnce(baseUser());

    renderPage();

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(["x"], "new.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());

    expect(callOrder).toEqual(["presign", "put", "updatePhoto"]);
    expect(presignMutateAsync).toHaveBeenCalledWith({
      filename: "new.png",
      content_type: "image/png",
    });
    expect(updatePhotoMutateAsync).toHaveBeenCalledWith("avatars/u1/new.png");
  });

  it("shows an error toast and keeps the previous avatar on upload failure", async () => {
    presignMutateAsync.mockResolvedValueOnce({
      url: "https://upload.example/put",
      method: "PUT",
      key: "avatars/u1/new.png",
    });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: false, status: 500 });

    renderPage();

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(["x"], "new.png", { type: "image/png" });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(toastError).toHaveBeenCalled());

    expect(updatePhotoMutateAsync).not.toHaveBeenCalled();
  });
});
