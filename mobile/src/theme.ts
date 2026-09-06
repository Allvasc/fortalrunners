// Paletas do manual de design — claro e escuro.
import { Easing } from "react-native";

export const light = {
  bg: "#F4F1E8",
  surface: "#FFFFFF",
  sunken: "#ECE6D9",
  ink: "#16262A",
  ink2: "#46565A",
  ink3: "#7B8688",
  line: "#CFC7B4",
  teal: "#0C7F86",
  tealDeep: "#0A5A61",
  coral: "#E0562F",
  gold: "#C98F2E",
  runner: "#08A6A0",
  good: "#2E8F63",
  crit: "#C7402B",
};

export const dark: typeof light = {
  bg: "#0E1618",
  surface: "#1B292C",
  sunken: "#0A1214",
  ink: "#E9E4D7",
  ink2: "#A8B1B1",
  ink3: "#7C8888",
  line: "#2C3A3E",
  teal: "#3CB4BE",
  tealDeep: "#6FD0D8",
  coral: "#F06A42",
  gold: "#DBA94A",
  runner: "#22C7C0",
  good: "#43B27D",
  crit: "#E05640",
};

export type Palette = typeof light;

// `C` legado = paleta clara (uso fora de componentes). Em componentes, use useTheme().
export const C = light;

// Movimento — manual de design §"Movimento & animação".
export const DUR = { xs: 120, sm: 200, md: 300, lg: 450, xl: 900 };
export const EASE = {
  standard: Easing.bezier(0.2, 0, 0, 1),
  out: Easing.bezier(0, 0, 0, 1),
  in: Easing.bezier(0.3, 0, 1, 1),
  spring: Easing.bezier(0.2, 0.9, 0.2, 1.12),
};
