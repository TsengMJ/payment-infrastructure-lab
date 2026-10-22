import Link from "next/link";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { logoutInternal } from "@/app/actions";
import { currentInternal } from "@/lib/session";

export default async function InternalLayout({ children }: { children: ReactNode }) {
  const user = await currentInternal();
  if (!user) {
    redirect("/internal/login");
  }
  return (
    <div className="shell">
      <header>
        <div>
          <strong>內部營運</strong>
          <div>{user.name}</div>
        </div>
        <nav>
          <Link href="/internal/kyb">審核</Link>
          <Link href="/internal/support">客服</Link>
          <Link href="/internal/audit">稽核</Link>
          <form action={logoutInternal}>
            <button type="submit">登出</button>
          </form>
        </nav>
      </header>
      <main>{children}</main>
    </div>
  );
}
