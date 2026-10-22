"use client";

import Link from "next/link";
import { useActionState } from "react";

import { completeAction, confirmAction } from "@/app/actions";
import type { ActionState, CheckoutSession } from "@/lib/types";

const INITIAL: ActionState = { error: "" };

export function CheckoutForm({ session, mockMode }: { session: CheckoutSession; mockMode: boolean }) {
  const [confirmState, confirm, confirmPending] = useActionState(confirmAction, INITIAL);
  const [completeState, complete, completePending] = useActionState(completeAction, INITIAL);
  const pending = confirmPending || completePending;
  const error = confirmState.error || completeState.error;

  return (
    <div className="stack">
      {session.status === "expired" ? <p>結帳已過期</p> : null}
      {session.status === "succeeded" || session.status === "failed" || session.status === "processing" ? (
        <p>
          <Link href={`/c/${session.id}/result`}>查看結果</Link>
        </p>
      ) : null}
      <fieldset className="methods">
        <legend>支付方式</legend>
        {session.methods.map((method, index) => (
          <label key={method.kind} data-testid={`method-${method.kind}`}>
            <input type="radio" name="method" value={method.kind} defaultChecked={index === 0} />
            {method.label}
          </label>
        ))}
      </fieldset>
      <div className="psp-field" data-testid="psp-field">
        <p>卡號由付款服務商的欄位收集。</p>
        <p>這個頁面不接收卡號或 CVV。</p>
      </div>
      {mockMode && session.status === "open" ? (
        <form action={confirm} className="actions">
          <input type="hidden" name="sessionId" value={session.id} />
          <button type="submit" name="scenario" value="success" disabled={pending}>
            付款成功
          </button>
          <button type="submit" name="scenario" value="3ds" disabled={pending}>
            需要 3DS
          </button>
          <button type="submit" name="scenario" value="decline" disabled={pending}>
            拒絕
          </button>
          <button type="submit" name="scenario" value="processing" disabled={pending}>
            處理中
          </button>
        </form>
      ) : null}
      {session.status === "requires_action" ? (
        <form action={complete} className="challenge">
          <h2>持卡人驗證</h2>
          <p>完成後以伺服器查到的狀態為準。</p>
          <input type="hidden" name="sessionId" value={session.id} />
          <button type="submit" disabled={pending}>
            完成驗證
          </button>
        </form>
      ) : null}
      {error ? <p role="alert">{error}</p> : null}
    </div>
  );
}
