import type { components } from "@/api/schema";

export type Space = components["schemas"]["Space"];
export type Share = components["schemas"]["Share"];
export type ShareInput = components["schemas"]["ShareInput"];
/** A built-in role (read, limited, editor, full) or "custom:<id>" (see /custom-roles). */
export type ShareRole = string;
export type Group = components["schemas"]["Group"];
export type User = components["schemas"]["User"];

/** Lowest to highest; mirrors the share role table in ARCHITECTURE §6.1. */
export const SHARE_ROLES: ShareRole[] = ["read", "limited", "editor", "full"];
export const isCustomRole = (role: ShareRole) => role.startsWith("custom:");
export const roleRank = (role: ShareRole) => SHARE_ROLES.indexOf(role);

export const spaceKeys = { all: ["spaces"] as const, one: (id: string) => ["spaces", id] as const };

/** Personal space first, the rest in server order. */
export function sortSpaces(spaces: Space[]) {
  return [...spaces.filter((s) => s.kind === "personal"), ...spaces.filter((s) => s.kind !== "personal")];
}

export const etag = (space: Pick<Space, "version">) => `"${space.version}"`;

export const can = (space: Space, capability: components["schemas"]["Capability"]) => !!space.my_capabilities?.includes(capability);

/**
 * Effective own role in a draft: the highest role among rows that reach the user directly or through a group.
 * Returns -1 when the user has no row at all.
 */
export function ownRank(rows: Pick<ShareInput, "principal_type" | "principal_id" | "role">[], userId: string, groups: Group[]) {
  const mine = new Set(groups.filter((g) => g.member_ids.includes(userId)).map((g) => g.id));
  return Math.max(-1, ...rows
    .filter((r) => (r.principal_type === "user" ? r.principal_id === userId : mine.has(r.principal_id)))
    .map((r) => roleRank(r.role)));
}

