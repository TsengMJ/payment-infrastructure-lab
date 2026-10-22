import { NextResponse } from "next/server";

import { getClient } from "@/lib/client";
import { currentMerchant } from "@/lib/session";

export async function GET() {
  const user = await currentMerchant();
  if (!user) {
    return new NextResponse("未登入", { status: 401 });
  }
  const report = await getClient().getSettlement();
  const header = "id,gross,fee,platform_fee,refund,dispute,net,currency";
  const lines = report.rows.map((row) =>
    [row.id, row.gross, row.fee, row.platformFee, row.refund, row.dispute, row.net, row.currency].join(","),
  );
  const body = `${[header, ...lines].join("\n")}\n`;
  return new NextResponse(body, {
    headers: { "content-type": "text/csv; charset=utf-8" },
  });
}
