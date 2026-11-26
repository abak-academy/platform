import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import AdminAuthLayout from "./layout";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));

describe("AdminAuthLayout", () => {
  it("renders the admin brand panel beside the form column", () => {
    render(<AdminAuthLayout><p>form</p></AdminAuthLayout>);
    expect(screen.getByText("ABAK ACADEMY · ADMIN")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Kelola platform dari satu tempat." })).toBeInTheDocument();
    expect(screen.queryByText(/pelajar/)).toBeNull();
    expect(screen.getByRole("main")).toHaveTextContent("form");
  });
});
