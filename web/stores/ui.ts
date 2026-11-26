import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Theme = "light" | "dark";
export type Lang = "id" | "en";

interface UIState {
  theme: Theme;
  lang: Lang;
  publicShells: number;
  enterPublic: () => void;
  leavePublic: () => void;
  toggleTheme: () => void;
  setLang: (lang: Lang) => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      theme: "light",
      lang: "id",
      publicShells: 0,
      enterPublic: () => set((s) => ({ publicShells: s.publicShells + 1 })),
      leavePublic: () => set((s) => ({ publicShells: Math.max(0, s.publicShells - 1) })),
      toggleTheme: () =>
        set((s) => ({ theme: s.theme === "light" ? "dark" : "light" })),
      setLang: (lang) => set({ lang }),
    }),
    {
      name: "abak-ui",
      partialize: (s) => ({ theme: s.theme, lang: s.lang }),
    }
  )
);
