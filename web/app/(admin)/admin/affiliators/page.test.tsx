import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import AdminAffiliatorsPage from "./page";

let allowed = true;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => allowed }));
vi.mock("@/components/admin/AffiliatorListTab", () => ({ AffiliatorListTab: () => <div>list tab</div> }));
vi.mock("@/components/admin/AffiliatorPromoteTab", () => ({ AffiliatorPromoteTab: () => <div>promote tab</div> }));

describe("AdminAffiliatorsPage", () => {
  beforeEach(() => {
    allowed = true;
  });

  it("has two tabs, the list first", () => {
    render(<AdminAffiliatorsPage />);

    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["Daftar affiliator", "Promote"]);
    expect(tabs[0]).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("list tab")).toBeInTheDocument();
    expect(screen.queryByText("promote tab")).not.toBeInTheDocument();
  });

  it("switches to the promote tab", () => {
    render(<AdminAffiliatorsPage />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Promote" }));

    expect(screen.getByText("promote tab")).toBeInTheDocument();
  });

  it("is limited to super admins", () => {
    allowed = false;
    render(<AdminAffiliatorsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
  });
});
