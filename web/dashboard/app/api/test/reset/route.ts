import { NextResponse } from "next/server";

import { resetStore } from "@/lib/mock/store";

export async function POST() {
  if (process.env.E2E_RESET !== "1") {
    return new NextResponse(null, { status: 404 });
  }
  resetStore();
  return new NextResponse(null, { status: 204 });
}
