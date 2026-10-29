"use client";

import { useState } from "react";

// Offset paging for reports. The offset is tied to resetKey, so changing the
// period, search or page size returns to the first page.
export function useOffsetPaging(resetKey: string, limit: number) {
  const [state, setState] = useState({ key: resetKey, offset: 0 });
  const offset = state.key === resetKey ? state.offset : 0;
  return {
    offset,
    canPrev: offset > 0,
    next: () => setState({ key: resetKey, offset: offset + limit }),
    prev: () => setState({ key: resetKey, offset: Math.max(0, offset - limit) }),
  };
}
