"use client";

import { useState } from "react";
import { toast } from "sonner";
import { useUpdateOwnAvatar } from "@/lib/hooks/profile";
import { useTranslation } from "@/lib/i18n";
import { AbakAvatar } from "@/components/ui/abak-avatar";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const AVATAR_COUNT = 35;

export function AvatarPicker({ name, current }: { name: string; current?: number | null }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const update = useUpdateOwnAvatar();

  function pick(index: number | null) {
    update.mutate(index, {
      onSuccess: () => toast.success(t("saved")),
      onError: (err) => toast.error(err instanceof Error ? err.message : t("save_failed")),
    });
  }

  function onKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    const step = { ArrowRight: 1, ArrowDown: 5, ArrowLeft: -1, ArrowUp: -5 }[e.key];
    if (!step) return;
    const radios = Array.from(e.currentTarget.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
    const at = radios.indexOf(document.activeElement as HTMLButtonElement);
    if (at < 0) return;
    e.preventDefault();
    const next = Math.min(Math.max(at + step, 0), radios.length - 1);
    radios[next].focus();
    pick(next);
  }

  return (
    <div className="flex w-full flex-col gap-2">
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-controls="avatar-picker-grid"
      >
        {t("avatar_pick")}
      </Button>
      {open && (
        <div id="avatar-picker-grid" className="flex flex-col gap-2">
          <div
            role="radiogroup"
            aria-label={t("avatar_pick")}
            onKeyDown={onKeyDown}
            className="grid max-h-[210px] grid-cols-5 gap-2 overflow-y-auto p-1"
          >
            {Array.from({ length: AVATAR_COUNT }, (_, i) => (
              <button
                key={i}
                type="button"
                aria-label={t("avatar_pick_item").replace("{n}", String(i + 1))}
                role="radio"
                aria-checked={current === i}
                tabIndex={i === (current ?? 0) ? 0 : -1}
                disabled={update.isPending}
                onClick={() => pick(i)}
                className={cn(
                  "rounded-full p-0.5 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-300",
                  current === i && "ring-2 ring-brand-600"
                )}
              >
                <AbakAvatar name={name} index={i} size="md" ring={false} />
              </button>
            ))}
          </div>
          {current != null && (
            <Button type="button" variant="ghost" size="sm" disabled={update.isPending} onClick={() => pick(null)}>
              {t("avatar_pick_reset")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
