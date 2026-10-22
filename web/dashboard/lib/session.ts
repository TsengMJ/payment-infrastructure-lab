import "server-only";

import { cookies } from "next/headers";

import { getClient } from "./client";

const COOKIE = {
  httpOnly: true,
  sameSite: "lax" as const,
  path: "/",
};

export async function currentMerchant() {
  const token = (await cookies()).get("merchant_session")?.value;
  if (!token) {
    return null;
  }
  return getClient().merchantFromToken(token);
}

export async function currentInternal() {
  const token = (await cookies()).get("internal_session")?.value;
  if (!token) {
    return null;
  }
  return getClient().internalFromToken(token);
}

export async function setMerchantCookie(token: string) {
  const cookieStore = await cookies();
  cookieStore.set("merchant_session", token, COOKIE);
}

export async function setInternalCookie(token: string) {
  const cookieStore = await cookies();
  cookieStore.set("internal_session", token, COOKIE);
}

export async function clearMerchantCookie() {
  const cookieStore = await cookies();
  cookieStore.delete("merchant_session");
}

export async function clearInternalCookie() {
  const cookieStore = await cookies();
  cookieStore.delete("internal_session");
}
