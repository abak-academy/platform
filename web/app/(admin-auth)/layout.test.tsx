import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import AdminAuthLayout from "./layout";
import { useAuthStore } from "@/stores/auth";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

beforeEach(() => {
  replace.mockReset();
  useAuthStore.setState({ token: null } as never);
});

describe("AdminAuthLayout", () => {
  it("renders the admin brand panel beside the form column", () => {
    render(<AdminAuthLayout><p>form</p></AdminAuthLayout>);
    expect(screen.getByText("ABAK ACADEMY · ADMIN")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Kelola platform dari satu tempat." })).toBeInTheDocument();
    expect(screen.queryByText(/pelajar/)).toBeNull();
    expect(screen.getByRole("main")).toHaveTextContent("form");
  });

  it("sends a signed-in user away and shows only the loading text", () => {
    useAuthStore.setState({ token: "t" } as never);
    render(<AdminAuthLayout><p>form</p></AdminAuthLayout>);
    expect(replace).toHaveBeenCalledWith("/");
    expect(screen.queryByText("form")).toBeNull();
  });
});
