import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppHeader } from "./AppHeader";
import type { ResolvedRole } from "@/lib/hooks/use-capability";
import { ADMIN_ROLES } from "@/lib/nav-config";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
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

let roleState: ResolvedRole = {
  role: "student",
  hydrated: true,
  meIsError: false,
};

vi.mock("@/lib/hooks/use-capability", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/hooks/use-capability")>()),
  useResolvedRole: () => roleState,
}));

async function openDropdownAndGetProfileLink() {
  render(<AppHeader />);
  const trigger = screen.getByRole("button", { name: /account/i });
  await userEvent.click(trigger);
  return screen.findByRole("menuitem", { name: /nav_profile/i });
}

describe("AppHeader — account dropdown profile link", () => {
  beforeEach(() => {
    roleState = { role: "student", hydrated: true, meIsError: false };
  });

  it.each(ADMIN_ROLES)("links to /admin/profile for %s", async (role) => {
    roleState = { role, hydrated: true, meIsError: false };
    const link = await openDropdownAndGetProfileLink();
    expect(link).toHaveAttribute("href", "/admin/profile");
  });

  it("links to /profile for student", async () => {
    roleState = { role: "student", hydrated: true, meIsError: false };
    const link = await openDropdownAndGetProfileLink();
    expect(link).toHaveAttribute("href", "/profile");
  });
});

describe("AppHeader — logout destination", () => {
  beforeEach(() => {
    replace.mockClear();
    logoutMutate.mockClear();
    roleState = { role: "student", hydrated: true, meIsError: false };
  });

  async function clickLogout() {
    render(<AppHeader />);
    await userEvent.click(screen.getByRole("button", { name: /account/i }));
    await userEvent.click(await screen.findByRole("menuitem", { name: /logout/i }));
  }

  it.each(ADMIN_ROLES)("sends %s to the admin login, not the student one", async (role) => {
    roleState = { role, hydrated: true, meIsError: false };

    await clickLogout();

    expect(logoutMutate).toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith("/admin/login");
    expect(replace).not.toHaveBeenCalledWith("/login");
  });

  it("still sends a student to the student login", async () => {
    roleState = { role: "student", hydrated: true, meIsError: false };

    await clickLogout();

    expect(replace).toHaveBeenCalledWith("/login");
  });
});
