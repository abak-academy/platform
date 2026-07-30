import type { OrderItem, ProductType } from "@/lib/types";

export function isPhysicalType(productType: ProductType): boolean {
  return ["book", "merchandise", "medal"].includes(productType);
}

export function isDigitalType(productType: ProductType): boolean {
  return !isPhysicalType(productType);
}

export function hasPhysicalItems(items: OrderItem[]): boolean {
  return items.some((item) => isPhysicalType(item.product_type as ProductType));
}

export function calculateTotalPhysicalWeight(items: OrderItem[]): number {
  return items
    .filter((item) => isPhysicalType(item.product_type as ProductType))
    .reduce((total, item) => total + (item.weight_grams ?? 0) * item.qty, 0);
}
