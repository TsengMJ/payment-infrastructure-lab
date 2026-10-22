import { getClient } from "@/lib/client";

export default async function AuditPage() {
  const entries = await getClient().listAudit();
  return (
    <section className="stack">
      <h1>稽核紀錄</h1>
      <table>
        <thead>
          <tr>
            <th>時間</th>
            <th>操作者</th>
            <th>動作</th>
            <th>之前</th>
            <th>之後</th>
            <th>理由</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <tr key={entry.id}>
              <td>{entry.at}</td>
              <td>{entry.actor}</td>
              <td>{entry.action}</td>
              <td>{entry.before}</td>
              <td>{entry.after}</td>
              <td>{entry.reason}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
