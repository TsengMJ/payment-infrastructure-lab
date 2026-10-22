import Link from "next/link";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { logoutMerchant } from "@/app/actions";
import { ROLE_LABEL } from "@/lib/roles";
import { currentMerchant } from "@/lib/session";

export default async function MerchantLayout({ children }: { children: ReactNode }) {
  const user = await currentMerchant();
  if (!user) {
    redirect("/login");
  }
  return (
    <div className="shell">
      <header>
        <div>
          <strong>{user.merchantName}</strong>
          <div data-testid="role">{ROLE_LABEL[user.role]}</div>
        </div>
        <nav>
          <Link href="/">付款</Link>
          <Link href="/reports">報表</Link>
          <Link href="/webhooks">Webhook</Link>
          <form action={logoutMerchant}>
            <button type="submit">登出</button>
          </form>
        </nav>
      </header>
      <main>{children}</main>
    </div>
  );
}
