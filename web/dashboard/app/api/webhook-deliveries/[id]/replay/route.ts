import { NextResponse } from "next/server";

import { getClient } from "@/lib/client";
import { PermissionError } from "@/lib/errors";
import { currentMerchant } from "@/lib/session";

export async function POST(_request: Request, context: { params: Promise<{ id: string }> }) {
  const user = await currentMerchant();
  if (!user) {
    return NextResponse.json({ error: "未登入" }, { status: 401 });
  }
  try {
    const { id } = await context.params;
    const delivery = await getClient().replayDelivery(user, id);
    return NextResponse.json(delivery);
  } catch (error) {
    if (error instanceof PermissionError) {
      return NextResponse.json({ error: error.message }, { status: 403 });
    }
    throw error;
  }
}
