import { getClient } from "@/lib/client";
import { formatMoney } from "@/lib/money";

export default async function ReportsPage() {
  const report = await getClient().getSettlement();
  return (
    <section className="stack">
      <h1>結算報表</h1>
      <p>{report.periodLabel}</p>
      <table>
        <thead>
          <tr>
            <th>編號</th>
            <th>gross</th>
            <th>手續費</th>
            <th>平台費</th>
            <th>退款</th>
            <th>爭議</th>
            <th>net</th>
          </tr>
        </thead>
        <tbody>
          {report.rows.map((row) => (
            <tr key={row.id}>
              <td>{row.id}</td>
              <td>{formatMoney(row.gross, row.currency)}</td>
              <td>{formatMoney(row.fee, row.currency)}</td>
              <td>{formatMoney(row.platformFee, row.currency)}</td>
              <td>{formatMoney(row.refund, row.currency)}</td>
              <td>{formatMoney(row.dispute, row.currency)}</td>
              <td data-testid="row-net">{row.net}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p>
        本期出款 <span data-testid="payout-amount">{report.payoutAmount}</span> {report.currency}
      </p>
      <p>
        <a href="/api/reports/settlements">下載 CSV</a>
      </p>
    </section>
  );
}
