"use client";

import { useState } from "react";
import { KeyRound } from "lucide-react";
import { useChangePassword } from "@/lib/hooks/students";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";

interface FieldErrors {
  old_password?: string;
  new_password?: string;
  confirm?: string;
}

const MIN_LENGTH = 8;

export function ChangePasswordForm({ onDone }: { onDone?: () => void } = {}) {
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [submitted, setSubmitted] = useState(false);

  const mutation = useChangePassword();

  function validate(): FieldErrors {
    const next: FieldErrors = {};
    if (!oldPassword) next.old_password = "Kata sandi lama wajib diisi.";
    if (newPassword.length < MIN_LENGTH)
      next.new_password = `Kata sandi baru minimal ${MIN_LENGTH} karakter.`;
    if (confirm !== newPassword)
      next.confirm = "Konfirmasi tidak cocok dengan kata sandi baru.";
    return next;
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitted(true);
    const next = validate();
    setErrors(next);
    if (Object.keys(next).length > 0) return;

    mutation.mutate(
      { current_password: oldPassword, new_password: newPassword },
      {
        onSuccess: () => {
          toast.success("Kata sandi berhasil diperbarui.");
          setOldPassword("");
          setNewPassword("");
          setConfirm("");
          setErrors({});
          setSubmitted(false);
          onDone?.();
        },
        onError: (err) => {
          const message =
            err instanceof Error && err.message
              ? err.message
              : "Gagal mengubah kata sandi.";
          toast.error(message);
        },
      }
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor="old_password">Kata sandi lama</Label>
        <Input
          id="old_password"
          type="password"
          autoComplete="current-password"
          value={oldPassword}
          onChange={(e) => {
            setOldPassword(e.target.value);
            if (submitted) setErrors((p) => ({ ...p, old_password: undefined }));
          }}
          aria-invalid={submitted && !!errors.old_password}
        />
        {submitted && errors.old_password && (
          <p className="text-xs text-danger">{errors.old_password}</p>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="new_password">Kata sandi baru</Label>
        <Input
          id="new_password"
          type="password"
          autoComplete="new-password"
          value={newPassword}
          onChange={(e) => {
            setNewPassword(e.target.value);
            if (submitted) {
              const v = e.target.value;
              setErrors((p) => ({
                ...p,
                new_password:
                  v.length < MIN_LENGTH
                    ? `Kata sandi baru minimal ${MIN_LENGTH} karakter.`
                    : undefined,
                confirm:
                  confirm && confirm !== v
                    ? "Konfirmasi tidak cocok dengan kata sandi baru."
                    : p.confirm,
              }));
            }
          }}
          aria-invalid={submitted && !!errors.new_password}
        />
        {submitted && errors.new_password && (
          <p className="text-xs text-danger">{errors.new_password}</p>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="confirm_password">Konfirmasi kata sandi baru</Label>
        <Input
          id="confirm_password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => {
            setConfirm(e.target.value);
            if (submitted) {
              const v = e.target.value;
              setErrors((p) => ({
                ...p,
                confirm:
                  v !== newPassword
                    ? "Konfirmasi tidak cocok dengan kata sandi baru."
                    : undefined,
              }));
            }
          }}
          aria-invalid={submitted && !!errors.confirm}
        />
        {submitted && errors.confirm && (
          <p className="text-xs text-danger">{errors.confirm}</p>
        )}
      </div>

      <div className="flex items-center gap-2 pt-1">
        <Button
          type="submit"
          disabled={mutation.isPending}
          className="gap-2"
        >
          <KeyRound className="size-4" />
          {mutation.isPending ? "Menyimpan…" : "Ubah kata sandi"}
        </Button>
      </div>
    </form>
  );
}