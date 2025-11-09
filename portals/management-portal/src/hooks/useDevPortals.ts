import { useContext } from "react";
import { DevPortalContext } from "../context/DevPortalContext";
import type { DevPortalContextValue } from "../context/DevPortalContext";

export const useDevPortals = (): DevPortalContextValue => {
  const context = useContext(DevPortalContext);
  if (!context) {
    throw new Error("useDevPortals must be used within a DevPortalProvider");
  }
  return context;
};