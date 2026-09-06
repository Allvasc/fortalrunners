import { useSyncExternalStore } from "react";
import { isAuthed, onAuthChange } from "./api";

// Reage a login/logout sem prop drilling. `isAuthed` lê o localStorage;
// `onAuthChange` avisa quando os tokens mudam nesta aba.
export function useAuthed(): boolean {
  return useSyncExternalStore(onAuthChange, isAuthed, () => false);
}
