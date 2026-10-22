"use server";

import { redirect } from "next/navigation";

import { getClient } from "@/lib/client";
import { AuthError, NotFoundError, PermissionError } from "@/lib/errors";
import {
  clearInternalCookie,
  clearMerchantCookie,
  currentInternal,
  currentMerchant,
  setInternalCookie,
  setMerchantCookie,
} from "@/lib/session";
import type { ActionState } from "@/lib/types";

export async function loginMerchantAction(_prev: ActionState, formData: FormData): Promise<ActionState> {
  try {
    const result = await getClient().loginMerchant(
      String(formData.get("email") ?? ""),
      String(formData.get("password") ?? ""),
    );
    await setMerchantCookie(result.token);
  } catch (error) {
    if (error instanceof AuthError) {
      return { error: error.message };
    }
    throw error;
  }
  redirect("/");
}

export async function loginInternalAction(_prev: ActionState, formData: FormData): Promise<ActionState> {
  try {
    const result = await getClient().loginInternal(
      String(formData.get("email") ?? ""),
      String(formData.get("password") ?? ""),
    );
    await setInternalCookie(result.token);
  } catch (error) {
    if (error instanceof AuthError) {
      return { error: error.message };
    }
    throw error;
  }
  redirect("/internal/kyb");
}

export async function logoutMerchant(): Promise<void> {
  await clearMerchantCookie();
  redirect("/login");
}

export async function logoutInternal(): Promise<void> {
  await clearInternalCookie();
  redirect("/internal/login");
}

export async function replayDeliveryAction(formData: FormData): Promise<void> {
  const user = await currentMerchant();
  if (!user) {
    redirect("/login");
  }
  try {
    await getClient().replayDelivery(user, String(formData.get("deliveryId") ?? ""));
  } catch (error) {
    if (error instanceof PermissionError) {
      redirect(`/webhooks?error=${encodeURIComponent(error.message)}`);
    }
    throw error;
  }
  redirect("/webhooks");
}

export async function replayInternalAction(formData: FormData): Promise<void> {
  const user = await currentInternal();
  if (!user) {
    redirect("/internal/login");
  }
  try {
    await getClient().replayAsInternal(user, String(formData.get("deliveryId") ?? ""));
  } catch (error) {
    if (error instanceof PermissionError) {
      redirect(`/internal/support?error=${encodeURIComponent(error.message)}`);
    }
    throw error;
  }
  redirect("/internal/support");
}

export async function decideKybAction(formData: FormData): Promise<void> {
  const user = await currentInternal();
  if (!user) {
    redirect("/internal/login");
  }
  const decision = String(formData.get("decision") ?? "");
  if (decision !== "approved" && decision !== "rejected") {
    redirect("/internal/kyb?error=不支援的決定");
  }
  try {
    await getClient().decideKyb(user, String(formData.get("caseId") ?? ""), decision);
  } catch (error) {
    if (error instanceof PermissionError || error instanceof NotFoundError) {
      redirect(`/internal/kyb?error=${encodeURIComponent(error.message)}`);
    }
    throw error;
  }
  redirect("/internal/kyb");
}

export async function requestRefundAction(formData: FormData): Promise<void> {
  const user = await currentInternal();
  if (!user) {
    redirect("/internal/login");
  }
  const rawAmount = String(formData.get("amount") ?? "");
  if (!/^[0-9]+$/.test(rawAmount)) {
    redirect("/internal/support?error=金額必須是正整數");
  }
  try {
    await getClient().requestRefund(
      user,
      String(formData.get("paymentId") ?? ""),
      Number(rawAmount),
      String(formData.get("reason") ?? ""),
    );
  } catch (error) {
    if (error instanceof PermissionError || error instanceof NotFoundError) {
      redirect(`/internal/support?error=${encodeURIComponent(error.message)}`);
    }
    throw error;
  }
  redirect("/internal/support");
}

export async function approveRefundAction(formData: FormData): Promise<void> {
  const user = await currentInternal();
  if (!user) {
    redirect("/internal/login");
  }
  try {
    await getClient().approveRefund(user, String(formData.get("refundId") ?? ""));
  } catch (error) {
    if (error instanceof PermissionError || error instanceof NotFoundError) {
      redirect(`/internal/support?error=${encodeURIComponent(error.message)}`);
    }
    throw error;
  }
  redirect("/internal/support");
}
