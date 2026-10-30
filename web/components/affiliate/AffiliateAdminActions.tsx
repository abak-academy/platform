"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ChoiceGroup } from "@/components/affiliate/ChoiceGroup";
import { LEVEL_TONE } from "@/components/affiliate/LevelBadge";
import { STATUS_TONE } from "@/components/affiliate/StatusBadge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import {
  useChangeAffiliateLevel,
  useChangeAffiliateStatus,
  useCreateAffiliateAdjustment,
} from "@/lib/hooks/admin-affiliates";
import { useTranslation } from "@/lib/i18n";

const STATUS_TARGETS = ["active", "suspended", "deactivated"] as const;
const LEVELS = ["bronze", "silver", "gold"] as const;
const CATEGORIES = ["correction", "offline", "bonus", "other"] as const;

type Action = "status" | "level" | "adjust" | null;

interface Props {
  id: string;
  currentStatus: string;
  currentLevel: string;
}

function Field({ id, label, children }: { id: string; label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id}>{label}</Label>
      {children}
    </div>
  );
}

export function AffiliateAdminActions({ id, currentStatus, currentLevel }: Props) {
  const { t } = useTranslation();
  const [action, setAction] = useState<Action>(null);
  const [status, setStatus] = useState<string>(STATUS_TARGETS[0]);
  const [level, setLevel] = useState(currentLevel);
  const [category, setCategory] = useState<string>(CATEGORIES[0]);
  const [amount, setAmount] = useState("");
  const [deduct, setDeduct] = useState(false);
  const [note, setNote] = useState("");
  const [reason, setReason] = useState("");

  const changeStatus = useChangeAffiliateStatus();
  const changeLevel = useChangeAffiliateLevel();
  const adjust = useCreateAffiliateAdjustment();

  const close = () => {
    setAction(null);
    setReason("");
    setAmount("");
    setDeduct(false);
    setNote("");
  };
  const done = () => {
    toast.success(t("admin_aff_act_done"));
    close();
  };
  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));
  const invalid = () => toast.error(t("admin_aff_act_invalid"));

  const submit = () => {
    if (!reason.trim()) {
      toast.error(t("aff_reason_required"));
      return;
    }
    const opts = { onSuccess: done, onError: fail };
    if (action === "status") {
      changeStatus.mutate({ id, status, reason: reason.trim() }, opts);
    } else if (action === "level") {
      changeLevel.mutate({ id, level, reason: reason.trim() }, opts);
    } else if (action === "adjust") {
      const n = Number(amount);
      if (!Number.isFinite(n) || n === 0) return invalid();
      adjust.mutate({ id, category, amount: deduct ? -n : n, reason: reason.trim(), note: note.trim() || undefined }, opts);
    }
  };

  const titles: Record<Exclude<Action, null>, string> = {
    status: t("admin_aff_act_status"),
    level: t("admin_aff_act_level"),
    adjust: t("admin_aff_act_adjust"),
  };
  const pending = changeStatus.isPending || changeLevel.isPending || adjust.isPending;

  return (
    <>
      <div className="flex flex-wrap gap-2">
        {currentStatus !== "pending_approval" && currentStatus !== "rejected" && (
          <Button variant="outline" size="sm" className="border-warn/40 bg-warn/10 text-warn hover:bg-warn/20" onClick={() => setAction("status")}>
            {t("admin_aff_act_status")}
          </Button>
        )}
        <Button variant="outline" size="sm" className="border-brand-700/30 bg-brand-50 text-brand-700 hover:bg-brand-100" onClick={() => setAction("level")}>
          {t("admin_aff_act_level")}
        </Button>
        <Button variant="outline" size="sm" className="border-success/40 bg-success/10 text-success hover:bg-success/20" onClick={() => setAction("adjust")}>
          {t("admin_aff_act_adjust")}
        </Button>
      </div>

      <Dialog open={action !== null} onOpenChange={(open) => !open && close()}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{action ? titles[action] : ""}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            {action === "status" && (
              <Field id="aff-status" label={t("admin_aff_field_status")}>
                <ChoiceGroup
                  label={t("admin_aff_field_status")}
                  value={status}
                  onChange={setStatus}
                  choices={STATUS_TARGETS.map((v) => ({ value: v, label: affLabel(t, v), activeClass: STATUS_TONE[v] }))}
                />
              </Field>
            )}
            {action === "level" && (
              <Field id="aff-level" label={t("admin_aff_field_level")}>
                <ChoiceGroup
                  label={t("admin_aff_field_level")}
                  value={level}
                  onChange={setLevel}
                  choices={LEVELS.map((v) => ({ value: v, label: affLabel(t, v), activeClass: LEVEL_TONE[v] }))}
                />
              </Field>
            )}
            {action === "adjust" && (
              <>
                <Field id="aff-category" label={t("admin_aff_field_category")}>
                  <ChoiceGroup
                    label={t("admin_aff_field_category")}
                    value={category}
                    onChange={setCategory}
                    choices={CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) }))}
                  />
                </Field>
                <Field id="aff-amount" label={t("admin_aff_field_amount")}>
                  <ChoiceGroup
                    label={t("admin_aff_field_direction")}
                    value={deduct ? "deduct" : "add"}
                    onChange={(v) => setDeduct(v === "deduct")}
                    choices={[
                      { value: "add", label: t("admin_aff_dir_add"), activeClass: "bg-success/15 text-success" },
                      { value: "deduct", label: t("admin_aff_dir_deduct"), activeClass: "bg-danger/15 text-danger" },
                    ]}
                  />
                  <Input
                    id="aff-amount"
                    inputMode="numeric"
                    placeholder="0"
                    value={amount}
                    onChange={(e) => setAmount(e.target.value.replace(/\D/g, ""))}
                  />
                </Field>
                <Field id="aff-note" label={t("admin_aff_field_note")}>
                  <Input id="aff-note" value={note} onChange={(e) => setNote(e.target.value)} />
                </Field>
              </>
            )}
            <Field id="aff-reason" label={t("aff_reason_label")}>
              <textarea
                id="aff-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={3}
                className="w-full rounded-md border border-line bg-surface p-2 text-sm"
              />
            </Field>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={close} disabled={pending}>
              {t("aff_cancel_btn")}
            </Button>
            <Button onClick={submit} disabled={pending}>
              {t("aff_confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
