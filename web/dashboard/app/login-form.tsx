"use client";

import { useActionState } from "react";

import { DEMO_PASSWORD } from "@/lib/demo";
import type { ActionState } from "@/lib/types";

const INITIAL: ActionState = { error: "" };

export function LoginForm({
  action,
  accounts,
}: {
  action: (state: ActionState, formData: FormData) => Promise<ActionState>;
  accounts: Array<{ email: string; role: string }>;
}) {
  const [state, formAction, pending] = useActionState(action, INITIAL);
  return (
    <div className="stack">
      <form action={formAction} className="stack">
        <label>
          電子郵件
          <input name="email" type="email" autoComplete="username" required />
        </label>
        <label>
          密碼
          <input name="password" type="password" autoComplete="current-password" required />
        </label>
        <button type="submit" disabled={pending}>
          登入
        </button>
        {state.error ? <p role="alert">{state.error}</p> : null}
      </form>
      <p>示範密碼：{DEMO_PASSWORD}</p>
      <ul className="links">
        {accounts.map((account) => (
          <li key={account.email}>
            {account.email}（{account.role}）
          </li>
        ))}
      </ul>
    </div>
  );
}
