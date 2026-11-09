"use client";

import { Suspense } from "react";
import { AffiliateListView } from "@/components/affiliate/AffiliateListView";

export default function AdminAffiliatesPage() {
  return (
    <Suspense fallback={null}>
      <AffiliateListView />
    </Suspense>
  );
}
