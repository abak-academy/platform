"use client";

import { Suspense } from "react";
import { AffiliateListView } from "@/components/affiliate/AffiliateListView";

export default function AdminPartnersPage() {
  return (
    <Suspense fallback={null}>
      <AffiliateListView lockedProgram="partner" />
    </Suspense>
  );
}
