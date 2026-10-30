"use client";

import { Suspense, use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { affiliateApi } from "@/lib/api";
import { setAffRefCookie } from "@/lib/aff-ref";

export default function ReferralPage({ params }: { params: Promise<{ code: string }> }) {
  return (
    <Suspense fallback={null}>
      <ReferralRoute params={params} />
    </Suspense>
  );
}

function ReferralRoute({ params }: { params: Promise<{ code: string }> }) {
  const { code } = use(params);
  const router = useRouter();

  useEffect(() => {
    setAffRefCookie(code);
    affiliateApi
      .validateCode(code)
      .then((res) => {
        if (res.cookie_days) setAffRefCookie(code, res.cookie_days);
      })
      .catch(() => {});
    router.replace("/");
  }, [code, router]);

  return null;
}
