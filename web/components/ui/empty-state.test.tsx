import { render, screen } from "@testing-library/react";
import { Trophy } from "lucide-react";
import { describe, expect, it } from "vitest";
import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("renders title, text and action", () => {
    render(<EmptyState icon={Trophy} title="Kosong" text="Coba lagi" action={<button>Reset</button>} />);
    expect(screen.getByRole("heading", { name: "Kosong" })).toBeInTheDocument();
    expect(screen.getByText("Coba lagi")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reset" })).toBeInTheDocument();
  });

  it("omits text and action when not given", () => {
    const { container } = render(<EmptyState icon={Trophy} title="Kosong" />);
    expect(container.querySelector("p")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });
});
