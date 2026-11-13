"use client";

import { useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { Layers } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminProductGroupings,
  useCreateProductGrouping,
  useUpdateProductGrouping,
  useDeleteProductGrouping,
} from "@/lib/hooks/admin-product-groupings";
import { useTranslation } from "@/lib/i18n";
import { ProductGroupingModal } from "@/components/admin/ProductGroupingModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import type {
  ProductGrouping,
  AdminCreateProductGroupingInput,
  AdminUpdateProductGroupingInput,
} from "@/lib/types";

function statusBadgeClass(status: string): string {
  return status === "published"
    ? "bg-green-100 text-green-800 border-green-200"
    : "bg-line-2 text-ink-700 border-line";
}

export default function ProductGroupingsPage() {
  const { t } = useTranslation();
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ProductGrouping | null>(null);

  const { data: groupings, isLoading, isError, error } = useAdminProductGroupings();
  const create = useCreateProductGrouping();
  const update = useUpdateProductGrouping();
  const remove = useDeleteProductGrouping();

  function openCreate() {
    setEditing(null);
    setModalOpen(true);
  }

  function openEdit(grouping: ProductGrouping) {
    setEditing(grouping);
    setModalOpen(true);
  }

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleSubmit(input: AdminCreateProductGroupingInput | AdminUpdateProductGroupingInput) {
    try {
      if (editing) {
        await update.mutateAsync({ id: editing.id, input: input as AdminUpdateProductGroupingInput });
        toast.success(t("changes_saved"));
      } else {
        await create.mutateAsync(input as AdminCreateProductGroupingInput);
        toast.success(t("grouping_created"));
      }
      setModalOpen(false);
      setEditing(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("grouping_confirm_delete"))) return;
    try {
      await remove.mutateAsync(id);
      toast.success(t("grouping_deleted"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Layers}
        title={t("grouping_page_title")}
        description={t("grouping_page_description")}
        actions={<Button onClick={openCreate}>{t("grouping_create")}</Button>}
      />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("grouping_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="overflow-x-auto md-card-outlined">
          <table className="w-full text-sm">
            <thead className="bg-muted">
              <tr>
                <th className="px-4 py-3 text-left font-medium">{t("th_name")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_status")}</th>
                <th className="px-4 py-3 text-right font-medium">{t("th_actions")}</th>
              </tr>
            </thead>
            <tbody>
              {groupings?.map((grouping) => (
                <tr key={grouping.id} className="border-t hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium">{grouping.name}</td>
                  <td className="px-4 py-3">
                    <Badge variant="outline" className={statusBadgeClass(grouping.status)}>
                      {t(grouping.status === "published" ? "grouping_status_published" : "grouping_status_draft")}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button size="sm" variant="outline" asChild>
                        <Link href={`/admin/product-groupings/${grouping.id}`}>{t("grouping_manage_products")}</Link>
                      </Button>
                      <Button size="sm" variant="outline" onClick={() => openEdit(grouping)}>
                        {t("action_edit")}
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => handleDelete(grouping.id)}
                        disabled={remove.isPending}
                      >
                        {t("action_delete")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {(groupings?.length ?? 0) === 0 && (
                <tr>
                  <td colSpan={3} className="px-4 py-8 text-center text-muted-foreground">
                    {t("empty_groupings")}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <ProductGroupingModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        grouping={editing}
        onSubmit={handleSubmit}
        isPending={create.isPending || update.isPending}
      />
    </div>
  );
}
