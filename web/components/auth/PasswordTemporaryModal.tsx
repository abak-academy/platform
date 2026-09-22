"use client";

import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useAuthStore } from "@/stores/auth";
import { useChangePassword } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";

// Shown to students whose password was created or reset by an admin
// (password_temporary from /auth/me). Dismissible — it reappears on the
// next login until the student sets their own password, which clears the
// server-side flag.
export function PasswordTemporaryModal({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { t } = useTranslation();
  const change = useChangePassword();
  const queryClient = useQueryClient();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");

  useEffect(() => {
    if (!open) {
      setCurrent("");
      setNext("");
      setConfirm("");
    }
  }, [open]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (next.length < 8) {
      toast.error(t("weak_password"));
      return;
    }
    if (next !== confirm) {
      toast.error(t("passwords_do_not_match"));
      return;
    }
    change.mutate(
      { current_password: current, new_password: next },
      {
        onSuccess: () => {
          toast.success(t("password_changed"));
          const { token, refreshToken, user } = useAuthStore.getState();
          if (token && user) {
            useAuthStore
              .getState()
              .setSession(token, refreshToken ?? "", {
                ...user,
                password_temporary: false,
              });
          }
          queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
          onOpenChange(false);
        },
        onError: (err) => {
          toast.error(
            err instanceof Error ? err.message : t("change_password_failed")
          );
        },
      }
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("password_temporary_title")}</DialogTitle>
          <DialogDescription>
            {t("password_temporary_body")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4 py-2">
          <p className="rounded-[10px] border border-line bg-surface px-3 py-2.5 text-[12.5px] text-ink-500">
            {t("password_temporary_hint")}
          </p>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="must-current-password">{t("current_password")}</Label>
            <PasswordInput
              id="must-current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              autoComplete="current-password"
              required
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="must-new-password">{t("new_password")}</Label>
            <PasswordInput
              id="must-new-password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              autoComplete="new-password"
              required
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="must-confirm-password">{t("confirm_password")}</Label>
            <PasswordInput
              id="must-confirm-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
              required
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("password_temporary_later")}
            </Button>
            <Button type="submit" disabled={change.isPending}>
              {t("update")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
