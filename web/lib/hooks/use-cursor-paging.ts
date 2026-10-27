"use client";

import { useState } from "react";

// Forward/back paging over a server cursor. The stack is tied to resetKey, so a
// new search or filter starts on page one without an effect round-trip.
export function useCursorPaging(resetKey: string) {
  const [state, setState] = useState<{ key: string; stack: string[] }>({ key: resetKey, stack: [""] });
  const stack = state.key === resetKey ? state.stack : [""];
  return {
    cursor: stack[stack.length - 1] || undefined,
    page: stack.length - 1,
    canPrev: stack.length > 1,
    next: (cursor: string) => setState({ key: resetKey, stack: [...stack, cursor] }),
    prev: () => setState({ key: resetKey, stack: stack.slice(0, -1) }),
  };
}
