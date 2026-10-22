import { replayDeliveryAction } from "@/app/actions";
import { getClient } from "@/lib/client";
import { canReplay } from "@/lib/roles";
import { currentMerchant } from "@/lib/session";

const STATUS_LABEL = {
  delivered: "已送達",
  failed: "失敗",
  replayed: "已重送",
} as const;

export default async function WebhooksPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const user = await currentMerchant();
  const params = await searchParams;
  const deliveries = await getClient().listDeliveries();
  const allowed = user ? canReplay(user.role) : false;
  return (
    <section className="stack">
      <h1>Webhook 投遞</h1>
      {params.error ? <p role="alert">{params.error}</p> : null}
      {allowed ? null : <p>你的角色不能重送。</p>}
      <table>
        <thead>
          <tr>
            <th>事件</th>
            <th>類型</th>
            <th>狀態</th>
            <th>次數</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {deliveries.map((delivery) => (
            <tr key={delivery.id}>
              <td>{delivery.eventId}</td>
              <td>{delivery.type}</td>
              <td>{STATUS_LABEL[delivery.status]}</td>
              <td>{delivery.attempts}</td>
              <td>
                {allowed ? (
                  <form action={replayDeliveryAction}>
                    <input type="hidden" name="deliveryId" value={delivery.id} />
                    <button type="submit">重送</button>
                  </form>
                ) : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
