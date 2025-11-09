import { useContext } from "react";
import { ApiPublishContext } from "../context/ApiPublishContext";

export const useApiPublishContext = () => {
  const ctx = useContext(ApiPublishContext);
  if (!ctx) {
    throw new Error("useApiPublishContext must be used within an ApiPublishProvider");
  }
  return ctx;
};