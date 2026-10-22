import Link from "next/link";

const LINKS = [
  ["/c/cs_success", "待付款"],
  ["/c/cs_3ds", "可走 3DS"],
  ["/c/cs_decline", "可拒絕"],
  ["/c/cs_processing_case", "可進入處理中"],
  ["/c/cs_expired", "已過期"],
  ["/c/cs_paid", "已付款"],
  ["/c/cs_processing/result", "處理中的結果"],
  ["/c/cs_jpy", "日圓訂單"],
  ["/pay/plink_once", "一次性付款連結"],
  ["/pay/plink_reusable", "可重複付款連結"],
] as const;

export default function HomePage() {
  return (
    <main>
      <h1>結帳示範</h1>
      <p>金額來自結帳 session，不從網址讀取。</p>
      <ul className="links">
        {LINKS.map(([href, label]) => (
          <li key={href}>
            <Link href={href}>{label}</Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
