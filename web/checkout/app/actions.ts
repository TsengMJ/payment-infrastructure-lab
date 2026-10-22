"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { getClient } from "@/lib/client";
import { CheckoutError } from "@/lib/errors";
import type { ActionState, ConfirmScenario } from "@/lib/types";

const SCENARIOS: readonly ConfirmScenario[] = ["success", "3ds", "decline", "processing"];

function isScenario(value: string): value is ConfirmScenario {
  return (SCENARIOS as readonly string[]).includes(value);
}

export async function confirmAction(_prev: ActionState, formData: FormData): Promise<ActionState> {
  const sessionId = String(formData.get("sessionId") ?? "");
  const scenario = String(formData.get("scenario") ?? "");
  if (!isScenario(scenario)) {
    return { error: "不支援的付款情境" };
  }
  try {
    const session = await getClient().confirm(sessionId, scenario);
    if (session.status !== "requires_action") {
      redirect(`/c/${sessionId}/result`);
    }
  } catch (error) {
    if (error instanceof CheckoutError) {
      return { error: error.message };
    }
    throw error;
  }
  revalidatePath(`/c/${sessionId}`);
  return { error: "" };
}

export async function completeAction(_prev: ActionState, formData: FormData): Promise<ActionState> {
  const sessionId = String(formData.get("sessionId") ?? "");
  try {
    await getClient().completeAction(sessionId);
  } catch (error) {
    if (error instanceof CheckoutError) {
      return { error: error.message };
    }
    throw error;
  }
  redirect(`/c/${sessionId}/result`);
}

export async function startFromLink(linkId: string): Promise<void> {
  const session = await getClient().startFromLink(linkId);
  redirect(`/c/${session.id}`);
}
