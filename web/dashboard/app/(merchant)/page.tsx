import Link from "next/link";

import { getClient } from "@/lib/client";
import { formatMoney } from "@/lib/money";

export default async function PaymentsPage() {
  const payments = await getClient().listPayments();
  return (
    <section className="stack">
      <h1>付款</h1>
      <table>
        <thead>
          <tr>
            <th>編號</th>
            <th>金額</th>
            <th>狀態</th>
            <th>摘要</th>
          </tr>
        </thead>
        <tbody>
          {payments.map((payment) => (
            <tr key={payment.id}>
              <td>
                <Link href={`/payments/${payment.id}`}>{payment.id}</Link>
              </td>
              <td>{formatMoney(payment.amount, payment.currency)}</td>
              <td>{payment.status}</td>
              <td>{payment.summary}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
