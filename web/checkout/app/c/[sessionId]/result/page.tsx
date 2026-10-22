import Link from "next/link";
import { notFound } from "next/navigation";

import { getClient } from "@/lib/client";
import { ApiNotReadyError } from "@/lib/errors";
import { formatMoney } from "@/lib/money";
import type { CheckoutSession, SessionStatus } from "@/lib/types";

const STATUS_LABEL: Record<SessionStatus, string> = {
  open: "尚未付款",
  requires_action: "等待驗證",
  processing: "處理中",
  succeeded: "付款成功",
  failed: "付款失敗",
  expired: "已過期",
};

export default async function ResultPage({ params }: { params: Promise<{ sessionId: string }> }) {
  const { sessionId } = await params;
  let unavailable = "";
  let session: CheckoutSession | null = null;
  try {
    session = await getClient().getSession(sessionId);
  } catch (error) {
    if (error instanceof ApiNotReadyError) {
      unavailable = error.message;
    } else {
      throw error;
    }
  }
  if (unavailable) {
    return (
      <main>
        <p role="alert">{unavailable}</p>
      </main>
    );
  }
  if (!session) {
    notFound();
  }
  return (
    <main>
      <section className="panel stack">
        <h1>付款結果</h1>
        <p data-testid="payment-status">{STATUS_LABEL[session.status]}</p>
        <p className="amount">{formatMoney(session.amount, session.currency)}</p>
        <p data-testid="payment-id">{session.paymentId ?? "尚無付款編號"}</p>
        {session.status === "processing" ? (
          <p>付款仍在處理。結果會由 webhook 通知商戶，這個頁面的網址不能當作付款成功。</p>
        ) : null}
        <p>
          <Link href={`/c/${session.id}`}>回到結帳</Link>
        </p>
      </section>
    </main>
  );
}
