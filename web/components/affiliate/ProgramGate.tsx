"use client";

import { Children, cloneElement, isValidElement, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { AlertTriangle, Clock, Info, XCircle } from "lucide-react";
import { useMe } from "@/lib/hooks/auth";
import { usePartnerMe } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import { Skeleton } from "@/components/ui/skeleton";

export type Program = "affiliate" | "partner";

const AFFILIATE_PATHS = ["/affiliate/overview", "/affiliate/students", "/affiliate/commissions", "/affiliate/vouchers", "/affiliate/wallet", "/affiliate/adjustments"];

// Which program a portal page belongs to; null for the pages both programs share.
export function programForPath(pathname: string): Program | null {
  if (pathname === "/affiliate/partner" || pathname.startsWith("/affiliate/partner/")) return "partner";
  if (AFFILIATE_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"))) return "affiliate";
  return null;
}

type Tone = "info" | "warn" | "danger";

const TONES: Record<Tone, { box: string; text: string; Icon: typeof Info }> = {
  info: { box: "border-info/30 bg-info-bg", text: "text-info", Icon: Info },
  warn: { box: "border-warn/30 bg-warn-bg", text: "text-warn", Icon: Clock },
  danger: { box: "border-danger/30 bg-danger-bg", text: "text-danger", Icon: XCircle },
};

function Notice({ title, description, tone }: { title: string; description: string; tone: Tone }) {
  const { box, text, Icon } = TONES[tone];
  return (
    <div role="status" data-tone={tone} className={`mx-auto mt-10 max-w-xl space-y-3 rounded-xl border p-8 text-center ${box}`}>
      <Icon className={`mx-auto size-8 ${text}`} aria-hidden />
      <h1 className={`font-serif text-xl font-bold ${text}`}>{title}</h1>
      <p className="text-sm leading-relaxed text-ink-700">{description}</p>
    </div>
  );
}

// ProgramGate shows a program's pages only to an account that belongs to it. An
// account that does not, or whose application is still open, sees why instead
// of data; the server refuses the same requests, so this is the explanation,
// not the lock.
export function ProgramGate({ program, children }: { program: Program; children: ReactNode }) {
  const { t } = useTranslation();
  const me = useMe();
  const states = me.data?.affiliator_programs;
  const state = states?.[program];
  const partnerMe = usePartnerMe(program === "partner" && state === "active");

  if (me.isLoading) {
    return <Skeleton className="h-40 w-full" data-testid="program-gate-loading" />;
  }
  if (!states || !state) return <>{children}</>;

  if (state === "not_enrolled") {
    return <Notice title={t("prog_not_enrolled_title")} description={t("prog_not_enrolled_desc")} tone="info" />;
  }
  if (state === "pending") {
    return <Notice title={t("prog_pending_title")} description={t("prog_pending_desc")} tone="warn" />;
  }
  if (state === "rejected") {
    const reason = states.rejection_reason ? ` ${t("prog_rejected_reason").replace("{reason}", states.rejection_reason)}` : "";
    return <Notice title={t("prog_rejected_title")} description={`${t("prog_rejected_desc")}${reason}`} tone="danger" />;
  }

  const programOff = program === "partner" && partnerMe.data?.program_enabled === false;
  return (
    <>
      {!states.account_active && (
        <div role="status" className="mb-4 flex items-start gap-3 rounded-lg border border-warn/30 bg-warn-bg px-5 py-3 text-sm text-warn">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>{t("prog_readonly_banner")}</p>
        </div>
      )}
      {programOff && (
        <div role="status" className="mb-4 flex items-start gap-3 rounded-lg border border-warn/30 bg-warn-bg px-5 py-3 text-sm text-warn">
          <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
          <p>{t("partner_program_off_banner")}</p>
        </div>
      )}
      {children}
    </>
  );
}

// RouteProgramGate picks the program from the URL, so one place in the portal
// layout covers every page of both programs.
export function RouteProgramGate({ children }: { children: ReactNode }) {
  const program = programForPath(usePathname());
  // The router can hand over its page as several elements without keys, which React
  // warns about even after Children.toArray; an explicit key on each one does not.
  const content = Children.toArray(children).map((child, i) => (isValidElement(child) ? cloneElement(child, { key: `route-${i}` }) : child));
  if (!program) return <>{content}</>;
  return <ProgramGate program={program}>{content}</ProgramGate>;
}
