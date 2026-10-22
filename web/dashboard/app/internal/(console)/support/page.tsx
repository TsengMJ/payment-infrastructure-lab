import { approveRefundAction, replayInternalAction, requestRefundAction } from "@/app/actions";
import { getClient } from "@/lib/client";
import { NotFoundError } from "@/lib/errors";
import { formatMoney } from "@/lib/money";
import { currentInternal } from "@/lib/session";
import type { TimelineEvent } from "@/lib/types";

export default async function SupportPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; payment?: string }>;
}) {
  const params = await searchParams;
  const user = await currentInternal();
  const client = getClient();
  const refunds = await client.listRefunds();
  const deliveries = await client.listDeliveries();
  let events: TimelineEvent[] = [];
  let lookupError = "";
  if (params.payment) {
    try {
      events = await client.getTimeline(params.payment);
    } catch (error) {
      if (error instanceof NotFoundError) {
        lookupError = error.message;
      } else {
        throw error;
      }
    }
  }
  return (
    <section className="stack">
      <h1>客服工具</h1>
      {params.error ? <p role="alert">{params.error}</p> : null}
      <form method="get" className="stack">
        <label>
          查詢付款編號
          <input name="payment" defaultValue={params.payment ?? ""} />
        </label>
        <button type="submit">查詢</button>
      </form>
      {lookupError ? <p role="alert">{lookupError}</p> : null}
      {events.length > 0 ? (
        <ol className="timeline">
          {events.map((event) => (
            <li key={event.id}>
              <strong>{event.title}</strong>
              <div>{event.detail}</div>
            </li>
          ))}
        </ol>
      ) : null}
      <h2>重送 webhook</h2>
      <ul className="timeline">
        {deliveries.map((delivery) => (
          <li key={delivery.id}>
            {delivery.eventId} {delivery.type} {delivery.status}
            {user?.canRefund ? (
              <form action={replayInternalAction}>
                <input type="hidden" name="deliveryId" value={delivery.id} />
                <button type="submit">重送</button>
              </form>
            ) : null}
          </li>
        ))}
      </ul>
      {user?.canRefund ? (
        <form action={requestRefundAction} className="stack">
          <h2>手動退款</h2>
          <label>
            付款編號
            <input name="paymentId" required />
          </label>
          <label>
            金額（最小貨幣單位）
            <input name="amount" inputMode="numeric" required />
          </label>
          <label>
            理由
            <input name="reason" required />
          </label>
          <button type="submit">送出退款</button>
        </form>
      ) : null}
      <h2>退款覆核</h2>
      <ul className="timeline">
        {refunds.map((refund) => (
          <li key={refund.id}>
            <span data-testid="refund-id">{refund.id}</span> {refund.paymentId}{" "}
            {formatMoney(refund.amount, refund.currency)} 發起人 {refund.makerName}{" "}
            <span>{refund.status === "approved" ? "已核准" : "待覆核"}</span>
            {refund.checkerName ? ` 覆核 ${refund.checkerName}` : null}
            {user?.canRefund && refund.status === "pending" ? (
              <form action={approveRefundAction}>
                <input type="hidden" name="refundId" value={refund.id} />
                <button type="submit">核准</button>
              </form>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}
