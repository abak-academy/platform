"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { ArrowLeft, ArrowDown, ArrowUp, X } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Layers } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminProductGrouping, useSetGroupingProducts } from "@/lib/hooks/admin-product-groupings";
import { useAdminProducts } from "@/lib/hooks/admin-products";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";

export default function ProductGroupingDetailPage() {
  const { t } = useTranslation();
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";

  const { data: grouping, isLoading, isError, error } = useAdminProductGrouping(id);
  const { data: allProducts } = useAdminProducts();
  const setMembers = useSetGroupingProducts();

  const [memberIds, setMemberIds] = useState<string[]>([]);
  const [search, setSearch] = useState("");

  useEffect(() => {
    if (grouping) setMemberIds(grouping.products.map((p) => p.id));
  }, [grouping]);

  const productsById = useMemo(() => {
    const map = new Map<string, { id: string; name: string; price: number }>();
    for (const p of allProducts ?? []) map.set(p.id, p);
    for (const p of grouping?.products ?? []) if (!map.has(p.id)) map.set(p.id, p);
    return map;
  }, [allProducts, grouping]);

  const members = memberIds.map((pid) => productsById.get(pid)).filter((p): p is { id: string; name: string; price: number } => Boolean(p));

  const candidates = useMemo(() => {
    const q = search.trim().toLowerCase();
    return (allProducts ?? []).filter((p) => !memberIds.includes(p.id) && (q === "" || p.name.toLowerCase().includes(q)));
  }, [allProducts, memberIds, search]);

  function move(index: number, delta: number) {
    const next = [...memberIds];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    setMemberIds(next);
  }

  function remove(productId: string) {
    setMemberIds(memberIds.filter((pid) => pid !== productId));
  }

  function add(productId: string) {
    setMemberIds([...memberIds, productId]);
    setSearch("");
  }

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleSave() {
    try {
      await setMembers.mutateAsync({ id, productIds: memberIds });
      toast.success(t("grouping_members_saved"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  return (
    <div className="space-y-6 fade-in">
      <Link href="/admin/product-groupings" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="size-4" aria-hidden />
        {t("grouping_back_to_list")}
      </Link>

      {isLoading && <Skeleton className="h-16 w-full" />}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("grouping_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {grouping && (
        <>
          <AdminPageHeader icon={Layers} title={grouping.name} description={grouping.description || undefined} />

          <div className="md-card-outlined space-y-4 p-4">
            <h2 className="text-title-medium font-medium">{t("grouping_members_title")}</h2>

            {members.length === 0 && <p className="text-sm text-muted-foreground">{t("grouping_members_empty")}</p>}

            {members.length > 0 && (
              <ul className="divide-y rounded-md border">
                {members.map((p, i) => (
                  <li key={p.id} className="flex items-center justify-between gap-3 px-3 py-2">
                    <div className="min-w-0 truncate">
                      <span className="font-medium">{p.name}</span>{" "}
                      <span className="text-xs text-muted-foreground">{formatRupiah(p.price)}</span>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="size-7"
                        aria-label={t("grouping_move_up")}
                        disabled={i === 0}
                        onClick={() => move(i, -1)}
                      >
                        <ArrowUp className="size-4" aria-hidden />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="size-7"
                        aria-label={t("grouping_move_down")}
                        disabled={i === members.length - 1}
                        onClick={() => move(i, 1)}
                      >
                        <ArrowDown className="size-4" aria-hidden />
                      </Button>
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="size-7"
                        aria-label={t("grouping_remove_product")}
                        onClick={() => remove(p.id)}
                      >
                        <X className="size-4" aria-hidden />
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}

            <div className="space-y-2">
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={t("grouping_add_product")}
              />
              {search.trim() !== "" && (
                <ul className="max-h-56 overflow-y-auto rounded-md border">
                  {candidates.length === 0 && (
                    <li className="px-3 py-2 text-sm text-muted-foreground">{t("grouping_add_product_empty")}</li>
                  )}
                  {candidates.map((p) => (
                    <li key={p.id}>
                      <button
                        type="button"
                        className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-muted"
                        onClick={() => add(p.id)}
                      >
                        <span className="min-w-0 truncate">{p.name}</span>
                        <span className="shrink-0 text-xs text-muted-foreground">{formatRupiah(p.price)}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={setMembers.isPending}>
                {t("grouping_save_members")}
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
