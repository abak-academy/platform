import { useEffect, useState } from "react";

export const SEARCH_DEBOUNCE_MS = 300;

export function useDebouncedValue(
  value: string,
  delay: number = SEARCH_DEBOUNCE_MS,
): string {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);

  return debounced;
}
