import React, { createContext, useContext, useEffect, useMemo, useState } from "react";

import type { ThemeMode } from "../types";

type ThemeContextValue = {
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = "tinybot.desktop.theme";

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<ThemeMode>(() => {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    return saved === "white" ? "white" : "black";
  });

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, theme);

    const root = document.getElementById("app");
    root?.setAttribute("data-theme", theme);
    document.body.setAttribute("data-theme", theme);
  }, [theme]);

  const value = useMemo(() => ({ theme, setTheme }), [theme]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const value = useContext(ThemeContext);
  if (!value) {
    throw new Error("useTheme must be used within ThemeProvider");
  }
  return value;
}
