// Tokens do manual de design (subset para o app).
export const C = {
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

// Movimento — manual de design §"Movimento & animação".
export const DUR = { xs: 120, sm: 200, md: 300, lg: 450, xl: 900 };
// Easings equivalentes aos cubic-bezier do manual (para Animated.timing).
import { Easing } from "react-native";
export const EASE = {
  standard: Easing.bezier(0.2, 0, 0, 1),
  out: Easing.bezier(0, 0, 0, 1),
  in: Easing.bezier(0.3, 0, 1, 1),
  spring: Easing.bezier(0.2, 0.9, 0.2, 1.12),
};
