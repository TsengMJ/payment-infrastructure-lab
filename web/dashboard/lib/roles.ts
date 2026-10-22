import type { Role } from "./types";

export const ROLE_LABEL: Record<Role, string> = {
  owner: "擁有者",
  admin: "管理員",
  developer: "開發者",
  support: "客服",
  viewer: "檢視者",
};

export function canReplay(role: Role): boolean {
  return role === "owner" || role === "admin" || role === "developer";
}
