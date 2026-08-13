"use client";

import { useCallback, useState } from "react";
import { Inbox, ShoppingBag } from "lucide-react";
import {
  useAdminNotifications,
  useMarkNotificationRead,
} from "@/lib/hooks/admin-notifications";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

function fmtDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function PurchaseNotificationFeed() {
  const { t } = useTranslation();
  const [unreadOnly, setUnreadOnly] = useState(false);

  const {
    items,
    isLoading,
    isError,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  } = useAdminNotifications({ unreadOnly });
  const markRead = useMarkNotificationRead();

  const unreadCount = items.filter((n) => !n.read).length;

  const handleMarkRead = useCallback(
    (id: string, alreadyRead: boolean) => {
      if (alreadyRead) return;
      markRead.mutate(id);
    },
    [markRead]
  );

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-[15px] font-semibold text-ink-900">
            {t("notification_inbox_title")}
          </h2>
          <p className="text-xs text-ink-600">{t("notification_inbox_description")}</p>
        </div>

        <div className="inline-flex rounded-full border border-line bg-surface-2 p-0.5">
          <button
            type="button"
            onClick={() => setUnreadOnly(false)}
            aria-pressed={!unreadOnly}
            className={cn(
              "rounded-full px-3 py-1 text-xs font-semibold transition-colors",
              unreadOnly ? "text-ink-600 hover:text-ink-900" : "bg-surface text-ink-900 shadow-sm"
            )}
          >
            {t("notification_show_all")}
          </button>
          <button
            type="button"
            onClick={() => setUnreadOnly(true)}
            aria-pressed={unreadOnly}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold transition-colors",
              unreadOnly ? "bg-surface text-ink-900 shadow-sm" : "text-ink-600 hover:text-ink-900"
            )}
          >
            {t("notification_unread_only")}
            {unreadCount > 0 && (
              <span className="inline-flex min-w-4 items-center justify-center rounded-full bg-brand-600 px-1 text-[10px] font-bold text-white tabular-nums">
                {unreadCount}
              </span>
            )}
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="overflow-hidden rounded-xl border border-line bg-surface divide-y divide-line">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3.5">
              <Skeleton className="size-9 shrink-0 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-3 w-56" />
              </div>
              <Skeleton className="h-4 w-20" />
            </div>
          ))}
        </div>
      ) : isError ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {t("notification_inbox_failed")}
        </div>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-line bg-surface px-6 py-12 text-center">
          <span className="flex size-12 items-center justify-center rounded-full bg-surface-2 text-ink-500">
            <Inbox className="size-6" />
          </span>
          <p className="text-sm font-medium text-ink-900">
            {unreadOnly ? t("notification_inbox_empty_unread") : t("notification_inbox_empty")}
          </p>
          {!unreadOnly && (
            <p className="max-w-sm text-xs text-ink-600">{t("notification_inbox_empty_hint")}</p>
          )}
        </div>
      ) : (
        <>
          <ul className="overflow-hidden rounded-xl border border-line bg-surface divide-y divide-line">
            {items.map((notif) => (
              <li key={notif.id}>
                <button
                  type="button"
                  onClick={() => handleMarkRead(notif.id, notif.read)}
                  disabled={notif.read}
                  aria-label={notif.read ? undefined : t("notification_mark_read")}
                  className={cn(
                    "flex w-full items-center gap-3 border-l-2 px-4 py-3.5 text-left transition-colors",
                    notif.read
                      ? "cursor-default border-l-transparent"
                      : "border-l-brand-600 bg-brand-50 hover:bg-brand-100"
                  )}
                >
                  <span
                    className={cn(
                      "flex size-9 shrink-0 items-center justify-center rounded-full",
                      notif.read ? "bg-surface-2 text-ink-500" : "bg-brand-100 text-brand-700"
                    )}
                  >
                    <ShoppingBag className="size-4" />
                  </span>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span
                        className={cn(
                          "truncate text-sm",
                          notif.read ? "font-medium text-ink-600" : "font-semibold text-ink-900"
                        )}
                      >
                        {notif.student_name}
                      </span>
                      {!notif.read && (
                        <span className="size-1.5 shrink-0 rounded-full bg-brand-600" />
                      )}
                    </div>
                    <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-ink-600">
                      <span className="font-mono text-[11px] text-ink-500">
                        {t("notification_order_label")} {notif.order_id}
                      </span>
                      <span className="text-ink-400">·</span>
                      <span>{fmtDateTime(notif.created_at)}</span>
                    </div>
                  </div>

                  <span
                    className={cn(
                      "shrink-0 text-sm tabular-nums",
                      notif.read ? "font-medium text-ink-600" : "font-semibold text-ink-900"
                    )}
                  >
                    {formatRupiah(notif.amount)}
                  </span>
                </button>
              </li>
            ))}
          </ul>

          {hasNextPage && (
            <div className="flex justify-center">
              <Button
                variant="outline"
                size="sm"
                onClick={() => fetchNextPage()}
                disabled={isFetchingNextPage}
              >
                {isFetchingNextPage ? t("notification_loading") : t("notification_load_more")}
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  );
}
