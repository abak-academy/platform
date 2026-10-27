import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { AffiliatorRegisterStudentModal } from "./AffiliatorRegisterStudentModal";

vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));
const toastError = vi.fn();
vi.mock("sonner", () => ({ toast: { error: (m: string) => toastError(m), success: vi.fn() } }));
const mutateAsync = vi.fn();
vi.mock("@/lib/hooks/admin-students", () => ({
  useRegisterStudent: () => ({ mutateAsync, isPending: false }),
}));

const schools = [
  { id: "s1", name: "SMAN 1" },
  { id: "s2", name: "SMAN 2" },
];

describe("AffiliatorRegisterStudentModal", () => {
  beforeEach(() => {
    mutateAsync.mockReset();
    toastError.mockReset();
  });

  it("registers into the chosen school and shows the credentials once", async () => {
    mutateAsync.mockResolvedValue({ id: "n1", username: "budi.1", temp_password: "Abc12345" });
    render(<AffiliatorRegisterStudentModal open onOpenChange={vi.fn()} schools={schools} />);

    fireEvent.change(screen.getByRole("combobox", { name: "Sekolah" }), { target: { value: "s2" } });
    fireEvent.change(screen.getByLabelText("Nama"), { target: { value: " Budi " } });
    fireEvent.change(screen.getByRole("combobox", { name: "Jenjang" }), { target: { value: "SMA" } });
    fireEvent.change(screen.getByLabelText("Email (opsional)"), { target: { value: "budi@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Daftarkan siswa" }));

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        schoolId: "s2",
        input: { name: "Budi", jenjang: "SMA", email: "budi@example.com" },
      }),
    );
    expect(await screen.findByText("budi.1")).toBeInTheDocument();
    expect(screen.getByText("Abc12345")).toBeInTheDocument();
  });

  it("preselects the only school and omits an empty email", async () => {
    mutateAsync.mockResolvedValue({ id: "n1", username: "siti.1", temp_password: "x" });
    render(<AffiliatorRegisterStudentModal open onOpenChange={vi.fn()} schools={[schools[0]]} />);

    expect(screen.getByRole("combobox", { name: "Sekolah" })).toHaveValue("s1");
    fireEvent.change(screen.getByLabelText("Nama"), { target: { value: "Siti" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Jenjang" }), { target: { value: "SMP" } });
    fireEvent.click(screen.getByRole("button", { name: "Daftarkan siswa" }));

    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({ schoolId: "s1", input: { name: "Siti", jenjang: "SMP" } }));
  });

  it("cannot submit without a school, a name and a level", () => {
    render(<AffiliatorRegisterStudentModal open onOpenChange={vi.fn()} schools={schools} />);

    expect(screen.getByRole("button", { name: "Daftarkan siswa" })).toBeDisabled();
  });

  it("reports a server refusal and stays on the form", async () => {
    mutateAsync.mockRejectedValue(new Error("school is not assigned to this affiliator"));
    render(<AffiliatorRegisterStudentModal open onOpenChange={vi.fn()} schools={[schools[0]]} />);

    fireEvent.change(screen.getByLabelText("Nama"), { target: { value: "Budi" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Jenjang" }), { target: { value: "SMA" } });
    fireEvent.click(screen.getByRole("button", { name: "Daftarkan siswa" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("school is not assigned to this affiliator"));
    expect(screen.getByLabelText("Nama")).toBeInTheDocument();
  });
});
