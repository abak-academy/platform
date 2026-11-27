"use client";

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useTranslation } from "@/lib/i18n";
import type { StudentCompetitionResult } from "@/lib/types";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  examTitle: string;
  result: StudentCompetitionResult;
  leaderboardHref: string | null;
}

export function CompetitionResultDialog({ open, onOpenChange, examTitle, result: r, leaderboardHref }: Props) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("stu_home_ext_title").replace("{name}", examTitle)}</DialogTitle>
        </DialogHeader>
        <div className="rounded-lg border border-line p-3">
          <p className="type-body-sm text-ink-600">
            {t("stu_home_ext_score")} {r.score}
            {r.rank != null && ` · ${t("stu_home_ext_rank")} #${r.rank}`}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {r.award && <Badge variant="gold">{r.award}</Badge>}
            {r.certificate_url && (
              <a
                href={r.certificate_url}
                target="_blank"
                rel="noopener noreferrer"
                className="type-label-sm text-brand-ink underline-offset-4 hover:underline"
              >
                {t("stu_home_ext_certificate")}
              </a>
            )}
          </div>
        </div>
        {leaderboardHref && (
          <Button asChild variant="outline" size="sm" className="self-start">
            <Link href={leaderboardHref}>{t("stu_home_ext_leaderboard")}</Link>
          </Button>
        )}
      </DialogContent>
    </Dialog>
  );
}
