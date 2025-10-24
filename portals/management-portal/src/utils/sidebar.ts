export type SidebarGroup = "apis" | "mcp" | "products";

/** Ask the sidebar to expand a specific group. */
export function openSidebarGroup(group: SidebarGroup) {
  window.dispatchEvent(new CustomEvent("open-submenu", { detail: { group } }));
}
