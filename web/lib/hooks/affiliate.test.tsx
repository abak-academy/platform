import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAffiliateMe,
  useAffiliateBindings,
  useAffiliateCommissions,
  useAffiliateLedger,
  useAffiliateStats,
  useUpdateAffiliateBank,
  useRequestAffiliatePayout,
  useAppealViolation,
  useValidateAffiliateCode,
  affiliateKeys,
} from "./affiliate";
import { useAuthStore } from "@/stores/auth";
import type { AffiliateProfile } from "@/lib/types";

const fetchMock = vi.fn();

const profile: AffiliateProfile = {
  code: "BUDI10",
  referral_link: "/r/BUDI10",
  level: "bronze",
  status: "active",
  warn_quarters: 0,
  bank_name: null,
  bank_account_number: null,
  bank_account_holder: null,
  terms_version: "2026-01",
  terms_accepted_at: "2026-01-01T00:00:00Z",
  level_progress: null,
};

function ok(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}

describe("affiliate hooks", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    useAuthStore.setState({ token: "tok", refreshToken: "rt", user: null });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    useAuthStore.setState({ token: null, refreshToken: null, user: null });
  });

  describe("useAffiliateMe", () => {
    it("gets /affiliates/me and returns the profile", async () => {
      fetchMock.mockResolvedValueOnce(ok(profile));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAffiliateMe(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me",
        expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer tok" }) }),
      );
      expect(result.current.data).toEqual(profile);
    });
  });

  describe("useUpdateAffiliateBank", () => {
    it("patches /affiliates/me/bank and invalidates the me query", async () => {
      fetchMock.mockResolvedValueOnce(ok({ ...profile, bank_name: "BCA" }));

      const { wrapper, queryClient } = wrapperFactory();
      const spy = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useUpdateAffiliateBank(), { wrapper });

      const data = await act(() =>
        result.current.mutateAsync({
          bank_name: "BCA",
          bank_account_number: "1234567890",
          bank_account_holder: "Budi",
        }),
      );

      expect(data.bank_name).toBe("BCA");
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/bank",
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({
            bank_name: "BCA",
            bank_account_number: "1234567890",
            bank_account_holder: "Budi",
          }),
        }),
      );
      expect(spy).toHaveBeenCalledWith({ queryKey: affiliateKeys.me() });
    });
  });

  describe("useAffiliateBindings", () => {
    it("gets /affiliates/me/bindings with the parsed list params", async () => {
      fetchMock.mockResolvedValueOnce(ok({ data: [], page: 2, per_page: 10, total: 0 }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(
        () => useAffiliateBindings({ page: 2, per_page: 10, status: "active" }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/bindings?page=2&per_page=10&status=active",
        expect.anything(),
      );
    });

    it("omits the query string when no params are given", async () => {
      fetchMock.mockResolvedValueOnce(ok({ data: [], page: 1, per_page: 20, total: 0 }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAffiliateBindings(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/bindings",
        expect.anything(),
      );
    });
  });

  describe("useAffiliateCommissions", () => {
    it("gets /affiliates/me/commissions with status and product_type filters", async () => {
      fetchMock.mockResolvedValueOnce(ok({ data: [], page: 1, per_page: 20, total: 0 }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(
        () => useAffiliateCommissions({ status: "pending", product_type: "book" }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/commissions?status=pending&product_type=book",
        expect.anything(),
      );
    });
  });

  describe("useAffiliateLedger", () => {
    it("gets /affiliates/me/ledger", async () => {
      fetchMock.mockResolvedValueOnce(ok({ data: [], saldo: 0, page: 1, per_page: 20, total: 0 }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAffiliateLedger(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/ledger",
        expect.anything(),
      );
    });
  });

  describe("useAffiliateStats", () => {
    it("sends the period as from and to", async () => {
      fetchMock.mockResolvedValueOnce(ok({ clicks: 0, registrations: 0, registrations_by_source: {}, conversion: 0, total_students: 0, active_bindings: 0, new_students: 0, commission_earned: 0 }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAffiliateStats({ from: "2026-10-01", to: "2026-10-20" }), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/stats?from=2026-10-01&to=2026-10-20",
        expect.anything(),
      );
    });

    it("gets /affiliates/me/stats", async () => {
      fetchMock.mockResolvedValueOnce(
        ok({
          clicks: 5,
          registrations: 2,
          registrations_by_source: { link: 1, manual_code: 1 },
          conversion: 0.4,
          total_students: 2,
          active_bindings: 1,
        }),
      );

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useAffiliateStats(), { wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/stats",
        expect.anything(),
      );
      expect(result.current.data?.registrations_by_source).toEqual({ link: 1, manual_code: 1 });
    });
  });

  describe("useRequestAffiliatePayout", () => {
    it("posts an explicit amount and invalidates the affiliate queries", async () => {
      fetchMock.mockResolvedValueOnce(ok({ id: "p1" }, 201));

      const { wrapper, queryClient } = wrapperFactory();
      const spy = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useRequestAffiliatePayout(), { wrapper });

      await act(() => result.current.mutateAsync({ amount: 50000 }));

      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/payouts",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ amount: 50000 }),
        }),
      );
      expect(spy).toHaveBeenCalledWith({ queryKey: affiliateKeys.all });
    });

    it("posts an empty body when no amount is given", async () => {
      fetchMock.mockResolvedValueOnce(ok({ id: "p1" }, 201));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useRequestAffiliatePayout(), { wrapper });

      await act(() => result.current.mutateAsync({}));

      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/payouts",
        expect.objectContaining({ method: "POST", body: "{}" }),
      );
    });
  });

  describe("useAppealViolation", () => {
    it("posts the appeal text to the violation route and invalidates violations", async () => {
      fetchMock.mockResolvedValueOnce(ok({ id: "v1", appeal_status: "submitted" }));

      const { wrapper, queryClient } = wrapperFactory();
      const spy = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useAppealViolation(), { wrapper });

      await act(() => result.current.mutateAsync({ id: "v1", text: "saya keberatan" }));

      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/me/violations/v1/appeal",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ text: "saya keberatan" }),
        }),
      );
      expect(spy).toHaveBeenCalledWith({
        queryKey: [affiliateKeys.all[0], "violations"],
      });
    });
  });

  describe("useValidateAffiliateCode", () => {
    it("gets the public validate endpoint with the encoded code", async () => {
      fetchMock.mockResolvedValueOnce(ok({ valid: true, affiliator_name: "Budi" }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useValidateAffiliateCode(), { wrapper });

      const data = await act(() => result.current.mutateAsync("ABC+1"));

      expect(fetchMock).toHaveBeenCalledWith(
        "http://localhost:8080/api/v1/affiliates/public/validate?code=ABC%2B1",
        expect.anything(),
      );
      expect(data).toEqual({ valid: true, affiliator_name: "Budi" });
    });

    it("surfaces valid:false without throwing", async () => {
      fetchMock.mockResolvedValueOnce(ok({ valid: false, affiliator_name: "" }));

      const { wrapper } = wrapperFactory();
      const { result } = renderHook(() => useValidateAffiliateCode(), { wrapper });

      const data = await act(() => result.current.mutateAsync("NOPE"));

      expect(data).toEqual({ valid: false, affiliator_name: "" });
    });
  });
});
