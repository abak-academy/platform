"use client";

import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { Package, Plus, Trash2 } from "lucide-react";
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
import { Badge, statusTone } from "@/components/ui/badge";
import { DataTable, TableItem } from "@/components/ui/data-table";
import { Switch } from "@/components/ui/switch";
import { TableFooter } from "@/components/ui/table-footer";
import { Skeleton } from "@/components/ui/skeleton";
import { formatRupiah } from "@/lib/format";
import type { Product, ProductType, AdminCreateProductInput, AdminUpdateProductInput } from "@/lib/types";
import { useResolvedRole } from "@/lib/hooks/use-capability";
import { writableProductTypes } from "@/lib/product-types";

const TYPE_TONE: Record<ProductType, "ember" | "gold" | "teal" | "sky" | "mute"> = {
  book: "gold", merchandise: "ember", medal: "gold", course: "teal", exam: "sky",
};

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
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("products_page_title")}
        description={t("products_page_description")}
        actions={<Button variant="secondary" onClick={openCreate}><Plus aria-hidden />{t("products_create")}</Button>}
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
          {t("products_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="min-w-0 overflow-hidden rounded-[var(--radius-card)] border border-line bg-surface shadow-card">
          <ProductsToolbar value={query} onChange={changeQuery} types={writableTypes} />
          <DataTable
            surface="plain"
            rows={rows}
            rowKey={(product) => product.id}
            empty={t("empty_products")}
            columns={[
              {
                key: "name",
                header: t("th_name"),
                strong: true,
                cell: (product) => (
                  <TableItem
                    thumb={<span className="grid size-[38px] place-items-center rounded-xl bg-ember-50 text-ember-700"><Package className="size-[18px]" aria-hidden /></span>}
                    title={product.name}
                  />
                ),
              },
              { key: "type", header: t("th_type"), cell: (product) => <Badge variant={TYPE_TONE[product.type]}>{filterLabel(product.type)}</Badge> },
              { key: "price", header: t("th_price"), align: "right", className: "tabular-nums", cell: (product) => formatRupiah(product.price) },
              { key: "stock", header: t("th_stock"), cell: (product) => formatStock(product) },
              {
                key: "status",
                header: t("th_status"),
                cell: (product) => {
                  const status = product.status ?? "draft";
                  return (
                    <span className="inline-flex items-center gap-2">
                      <Switch
                        size="sm"
                        checked={status === "published"}
                        disabled={status === "published" || publish.isPending}
                        aria-label={t("action_publish")}
                        onCheckedChange={() => handlePublish(product.id)}
                      />
                      {status !== "published" && status !== "draft" && <Badge variant={statusTone(status)}>{t(`products_status_${status}`)}</Badge>}
                    </span>
                  );
                },
              },
            ]}
            rowActions={(product) => (
              <div className="flex items-center justify-end gap-1.5">
                <Button size="sm" variant="outline" onClick={() => openEdit(product)}>
                  {t("action_edit")}
                </Button>
                <Button
                  size="icon-sm"
                  variant="ghost"
                  aria-label={t("action_delete")}
                  onClick={() => handleDelete(product.id)}
                  disabled={remove.isPending}
                >
                  <Trash2 aria-hidden />
                </Button>
              </div>
            )}
            footer={
              <TableFooter
                summary={`${t("products_showing")} ${firstRow}-${lastRow} ${t("products_of_total")} ${filtered.length}`}
                onPrev={() => setPageIndex(currentPage - 1)}
                onNext={() => setPageIndex(currentPage + 1)}
                hasPrev={currentPage > 1}
                hasNext={currentPage < lastPage}
              >
                <Select value={String(query.limit)} onValueChange={(v) => changeQuery({ ...query, limit: Number(v) as 20 | 50 | 100 })}>
                  <SelectTrigger size="sm" className="w-24" aria-label={t("products_page_size")}><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {[20, 50, 100].map((limit) => <SelectItem key={limit} value={String(limit)}>{limit}</SelectItem>)}
                  </SelectContent>
                </Select>
              </TableFooter>
            }
          />
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
