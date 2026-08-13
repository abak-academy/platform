import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { AppSidebar } from "./AppSidebar";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/dashboard",
}));

// Runs onSettled the way the real mutation does, so the redirect the component
// registers actually fires.
const logoutMutate = vi.fn(
  (_input: unknown, opts?: { onSettled?: () => void }) => opts?.onSettled?.()
);

vi.mock("@/lib/hooks/auth", () => ({
  useLogout: () => ({ mutate: logoutMutate }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

let authStore = {
  user: null as {
    name?: string;
    email?: string;
    username?: string;
    role?: string;
    photo_url?: string;
  } | null,
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

vi.mock("./AbakMark", () => ({
  AbakMark: () => <span data-testid="abak-mark" />,
}));

// Radix AvatarImage only renders <img> after native image load, which
// never fires in jsdom.  We mock the avatar UI to behave like a real
// browser: render <img> when src is provided, fallback otherwise.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children, className, ...props }: any) => (
    <div data-slot="avatar" className={className} {...props}>
      {children}
    </div>
  ),
  AvatarImage: ({ src, ...props }: { src?: string; [key: string]: any }) =>
    src ? <img src={src} alt="" data-slot="avatar-image" {...props} /> : null,
  AvatarFallback: ({ children, ...props }: any) => (
    <span data-slot="avatar-fallback" {...props}>
      {children}
    </span>
  ),
}));

describe("AppSidebar — avatar rendering", () => {
  beforeEach(() => {
    replace.mockClear();
    authStore = { user: null };
  });

  it("renders AvatarImage when user.photo_url is set", () => {
    authStore = {
      user: {
        name: "Budi Santoso",
        email: "budi@test.com",
        role: "student",
        photo_url: "https://example.com/photo.jpg",
      },
    };
    const { container } = render(<AppSidebar role="student" />);

    const imgs = container.querySelectorAll('img[data-slot="avatar-image"]');
    expect(imgs.length).toBeGreaterThan(0);
    const imgWithSrc = Array.from(imgs).find(
      (img) => img.getAttribute("src") === "https://example.com/photo.jpg"
    );
    expect(imgWithSrc).toBeTruthy();
  });

  it("falls back to AvatarFallback initials when photo_url is absent", () => {
    authStore = {
      user: {
        name: "Budi Santoso",
        email: "budi@test.com",
        role: "student",
        photo_url: undefined,
      },
    };
    render(<AppSidebar role="student" />);

    const imgs = document.querySelectorAll('img[data-slot="avatar-image"]');
    expect(imgs.length).toBe(0);
    expect(screen.getAllByText("B").length).toBeGreaterThan(0);
  });

  it("shows default initial 'A' when user has no name/email/username", () => {
    authStore = { user: { role: "student", photo_url: undefined } };
    render(<AppSidebar role="student" />);

    expect(screen.getByText("A")).toBeInTheDocument();
  });
});

describe("AppSidebar — logout destination", () => {
  beforeEach(() => {
    replace.mockClear();
    logoutMutate.mockClear();
    authStore = { user: null };
  });

  function clickLogout() {
    fireEvent.click(screen.getAllByLabelText("logout")[0]);
  }

  it("sends an admin to the admin login, not the student one", () => {
    authStore = { user: { name: "Admin", role: "super_admin" } };
    render(<AppSidebar role="super_admin" />);

    clickLogout();

    expect(logoutMutate).toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith("/admin/login");
    expect(replace).not.toHaveBeenCalledWith("/login");
  });

  it("sends every admin role to the admin login", () => {
    for (const role of ["admin_store", "admin_exam", "admin_school"] as const) {
      replace.mockClear();
      authStore = { user: { name: "Admin", role } };
      const { unmount } = render(<AppSidebar role={role} />);

      clickLogout();

      expect(replace, `${role} landed on the wrong login`).toHaveBeenCalledWith(
        "/admin/login"
      );
      unmount();
    }
  });

  it("still sends a student to the student login", () => {
    authStore = { user: { name: "Budi", role: "student" } };
    render(<AppSidebar role="student" />);

    clickLogout();

    expect(replace).toHaveBeenCalledWith("/login");
  });

  // It previously called mutate with no callback at all, leaving the tab where
  // it was until a route guard happened to bounce it.
  it("navigates on logout rather than leaving the tab in place", () => {
    authStore = { user: { name: "Admin", role: "super_admin" } };
    render(<AppSidebar role="super_admin" />);

    clickLogout();

    expect(replace).toHaveBeenCalledTimes(1);
  });
});
