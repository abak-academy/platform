"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Package } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminProducts,
  useCreateProduct,
  useUpdateProduct,
  usePublishProduct,
  useDeleteProduct,
} from "@/lib/hooks/admin-products";
import { useTranslation } from "@/lib/i18n";
import { ProductsToolbar, DEFAULT_PRODUCTS_QUERY, applyProductsQuery, productStock, type ProductsQuery } from "@/components/admin/ProductsToolbar";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ProductModal } from "@/components/admin/ProductModal";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { formatRupiah } from "@/lib/format";
import type { Product, ProductType, AdminCreateProductInput, AdminUpdateProductInput } from "@/lib/types";
import { useResolvedRole } from "@/lib/hooks/use-capability";
import { writableProductTypes } from "@/lib/product-types";

function typeBadgeClass(type: ProductType): string {
  switch (type) {
    case "book":
      return "bg-amber-100 text-amber-800 border-amber-200";
    case "merchandise":
      return "bg-orange-100 text-orange-800 border-orange-200";
    case "medal":
      return "bg-yellow-100 text-yellow-800 border-yellow-200";
    case "course":
      return "bg-green-100 text-green-800 border-green-200";
    case "exam":
      return "bg-blue-100 text-blue-800 border-blue-200";
  }
}

function statusBadgeClass(status?: string): string {
  switch (status) {
    case "published":
      return "bg-green-100 text-green-800 border-green-200";
    case "draft":
      return "bg-line-2 text-ink-700 border-line";
    case "hidden":
      return "bg-amber-100 text-amber-800 border-amber-200";
    case "archived":
      return "bg-red-100 text-red-800 border-red-200";
    default:
      return "bg-line-2 text-ink-700 border-line";
  }
}

function formatStock(product: Product): string | number {
  return productStock(product) ?? "-";
}

export default function ProductsPage() {
  const { t } = useTranslation();
  const [query, setQuery] = useState<ProductsQuery>(DEFAULT_PRODUCTS_QUERY);
  const [pageIndex, setPageIndex] = useState(1);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<Product | null>(null);

  const { data: products, isLoading, isError, error } = useAdminProducts();
  const { role } = useResolvedRole();
  const writableTypes = useMemo(() => writableProductTypes(role), [role]);
  const create = useCreateProduct();
  const update = useUpdateProduct();
  const publish = usePublishProduct();
  const remove = useDeleteProduct();

  const filtered = useMemo(() => {
    const visible = (products ?? []).filter((product) => writableTypes.includes(product.type));
    return applyProductsQuery(visible, query);
  }, [products, query, writableTypes]);
  const lastPage = Math.max(1, Math.ceil(filtered.length / query.limit));
  const currentPage = Math.min(pageIndex, lastPage);
  const offset = (currentPage - 1) * query.limit;
  const rows = filtered.slice(offset, offset + query.limit);
  const firstRow = filtered.length ? offset + 1 : 0;
  const lastRow = offset + rows.length;

  useEffect(() => {
    setPageIndex((index) => Math.min(index, lastPage));
  }, [lastPage]);

  function changeQuery(next: ProductsQuery) {
    setQuery(next);
    setPageIndex(1);
  }

  function openCreate() {
    setEditingProduct(null);
    setModalOpen(true);
  }

  function openEdit(product: Product) {
    setEditingProduct(product);
    setModalOpen(true);
  }

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleCreate(input: AdminCreateProductInput) {
    try {
      await create.mutateAsync(input);
      toast.success(t("products_created"));
      setModalOpen(false);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleUpdate(input: AdminUpdateProductInput) {
    if (!editingProduct) return;
    try {
      await update.mutateAsync({ id: editingProduct.id, input });
      toast.success(t("changes_saved"));
      setModalOpen(false);
      setEditingProduct(null);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleSubmit(input: AdminCreateProductInput | AdminUpdateProductInput) {
    if (editingProduct) {
      await handleUpdate(input as AdminUpdateProductInput);
    } else {
      await handleCreate(input as AdminCreateProductInput);
    }
  }

  async function handlePublish(id: string) {
    try {
      await publish.mutateAsync(id);
      toast.success(t("products_published"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("products_confirm_delete"))) return;
    try {
      await remove.mutateAsync(id);
      toast.success(t("products_deleted"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  const filterLabel = (type: ProductType): string => {
    switch (type) {
      case "book":
        return t("product_type_book");
      case "merchandise":
        return t("product_type_merchandise");
      case "medal":
        return t("product_type_medal");
      case "course":
        return t("product_type_course");
      case "exam":
        return t("product_type_exam");
    }
  };

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Package}
        title={t("products_page_title")}
        description={t("products_page_description")}
        actions={<Button onClick={openCreate}>{t("products_create")}</Button>}
      />

      <ProductsToolbar value={query} onChange={changeQuery} types={writableTypes} />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("products_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="space-y-4">
          <div className="overflow-x-auto md-card-outlined">
            <table className="w-full text-sm">
              <thead className="bg-muted">
                <tr>
                  <th className="px-4 py-3 text-left font-medium">{t("th_name")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("th_type")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("th_price")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("th_stock")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("th_status")}</th>
                  <th className="px-4 py-3 text-right font-medium">{t("th_actions")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((product) => (
                  <tr
                    key={product.id}
                    className="border-t transition-colors hover:bg-muted/40"
                  >
                    <td className="px-4 py-3 font-medium">{product.name}</td>
                    <td className="px-4 py-3">
                      <Badge className={typeBadgeClass(product.type)}>{filterLabel(product.type)}</Badge>
                    </td>
                    <td className="px-4 py-3">{formatRupiah(product.price)}</td>
                    <td className="px-4 py-3">{formatStock(product)}</td>
                    <td className="px-4 py-3">
                      <Badge className={statusBadgeClass(product.status)}>{product.status ?? "draft"}</Badge>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-2">
                        {product.status !== "published" && (
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handlePublish(product.id)}
                            disabled={publish.isPending}
                          >
                            {t("action_publish")}
                          </Button>
                        )}
                        <Button size="sm" variant="outline" onClick={() => openEdit(product)}>
                          {t("action_edit")}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => handleDelete(product.id)}
                          disabled={remove.isPending}
                        >
                          {t("action_delete")}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-muted-foreground">
                      {t("empty_products")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 px-1">
            <div className="flex items-center gap-2">
              <span className="text-sm text-ink-600">{t("products_page_size")}</span>
              <Select value={String(query.limit)} onValueChange={(v) => changeQuery({ ...query, limit: Number(v) as 20 | 50 | 100 })}>
                <SelectTrigger className="w-24" aria-label={t("products_page_size")}><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[20, 50, 100].map((limit) => <SelectItem key={limit} value={String(limit)}>{limit}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-wrap items-center gap-4">
              <p className="text-xs tabular-nums text-ink-600">
                {t("products_showing")} {firstRow}-{lastRow} {t("products_of_total")} {filtered.length}
              </p>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={currentPage === 1}
                  onClick={() => setPageIndex(currentPage - 1)}>
                  {t("pagination_prev")}
                </Button>
                <Button variant="outline" size="sm" disabled={currentPage === lastPage}
                  onClick={() => setPageIndex(currentPage + 1)}>
                  {t("pagination_next")}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      <ProductModal
        open={modalOpen}
        onOpenChange={setModalOpen}
        product={editingProduct}
        onSubmit={handleSubmit}
        isPending={create.isPending || update.isPending}
      />
    </div>
  );
}
