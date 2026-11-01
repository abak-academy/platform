import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // The balance history and payouts used to be two menus; keep their old links working.
  async redirects() {
    return [
      { source: "/affiliate/ledger", destination: "/affiliate/wallet", permanent: false },
      { source: "/affiliate/payouts", destination: "/affiliate/wallet?tab=payouts", permanent: false },
    ];
  },
};

export default nextConfig;
