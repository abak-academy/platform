import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { AffiliatorCreateDialog } from "./AffiliatorCreateDialog";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/admin/AffiliatorBulkPanel", () => ({ AffiliatorBulkPanel: () => <div>bulk panel</div> }));
vi.mock("@/components/admin/SchoolMultiPicker", () => ({
  SchoolMultiPicker: ({ value, onChange }: { value: { id: string; name: string }[]; onChange: (v: { id: string; name: string }[]) => void }) => (
    <button type="button" onClick={() => onChange([...value, { id: "s1", name: "SMAN 1" }])}>add school ({value.length})</button>
  ),
}));

const create = vi.fn();
vi.mock("@/lib/hooks/admin-affiliators", () => ({ useCreateAffiliator: () => ({ mutateAsync: create, isPending: false }) }));

function fillBasics() {
  fireEvent.change(screen.getByLabelText("Nama"), { target: { value: " Citra " } });
  fireEvent.change(screen.getByLabelText("Email"), { target: { value: "citra@example.com" } });
  fireEvent.click(screen.getByRole("button", { name: /add school/ }));
}

describe("AffiliatorCreateDialog", () => {
  beforeEach(() => {
    create.mockReset().mockResolvedValue({ id: "a1", email: "citra@example.com" });
    vi.mocked(toast.success).mockReset();
    vi.mocked(toast.error).mockReset();
  });

  it("offers one account or many by CSV, and starts on one", () => {
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);

    expect(screen.getByRole("radio", { name: "Satu akun" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Banyak akun (CSV)" })).not.toBeChecked();
    expect(screen.queryByText("bulk panel")).not.toBeInTheDocument();
  });

  it("colours the chosen mode and not the other, and moves the colour when switched", () => {
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);
    const single = screen.getByRole("radio", { name: "Satu akun" });
    const bulk = screen.getByRole("radio", { name: "Banyak akun (CSV)" });
    expect(single).toHaveAttribute("data-variant", "default");
    expect(bulk).toHaveAttribute("data-variant", "outline");

    fireEvent.click(bulk);

    expect(bulk).toHaveAttribute("data-variant", "default");
    expect(single).toHaveAttribute("data-variant", "outline");
  });

  it("shows the CSV upload only after choosing many", () => {
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole("radio", { name: "Banyak akun (CSV)" }));

    expect(screen.getByText("bulk panel")).toBeInTheDocument();
    expect(screen.queryByLabelText("Nama")).not.toBeInTheDocument();
  });

  it("needs a name, an email and a school before it can be created", () => {
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);
    const submit = within(screen.getByRole("dialog")).getByRole("button", { name: "Tambah affiliator" });
    expect(submit).toBeDisabled();

    fillBasics();
    expect(submit).toBeEnabled();
  });

  it("rejects a password shorter than 8 characters but allows leaving it empty", () => {
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);
    fillBasics();
    const submit = within(screen.getByRole("dialog")).getByRole("button", { name: "Tambah affiliator" });

    fireEvent.change(screen.getByLabelText(/Kata sandi/), { target: { value: "short" } });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/Kata sandi/), { target: { value: "" } });
    expect(submit).toBeEnabled();
  });

  it("creates with the given password and closes with a confirmation", async () => {
    const onClose = vi.fn();
    render(<AffiliatorCreateDialog open onClose={onClose} />);
    fillBasics();
    fireEvent.change(screen.getByLabelText(/Kata sandi/), { target: { value: "longenough1" } });
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Tambah affiliator" }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith({ name: "Citra", email: "citra@example.com", school_ids: ["s1"], password: "longenough1" }),
    );
    expect(toast.success).toHaveBeenCalledWith("Affiliator dibuat.");
    expect(onClose).toHaveBeenCalled();
  });

  it("omits the password when empty and shows the generated one once", async () => {
    create.mockResolvedValue({ id: "a1", email: "citra@example.com", temp_password: "Gen12345" });
    const onClose = vi.fn();
    render(<AffiliatorCreateDialog open onClose={onClose} />);
    fillBasics();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Tambah affiliator" }));

    await waitFor(() => expect(create).toHaveBeenCalledWith({ name: "Citra", email: "citra@example.com", school_ids: ["s1"] }));
    expect(await screen.findByText("Gen12345")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Tutup" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("reports a server refusal and keeps the form", async () => {
    create.mockRejectedValue(new Error("email already registered"));
    render(<AffiliatorCreateDialog open onClose={vi.fn()} />);
    fillBasics();
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Tambah affiliator" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("email already registered"));
    expect(screen.getByLabelText("Nama")).toHaveValue(" Citra ");
  });
});
