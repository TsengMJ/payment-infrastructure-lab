import { notFound } from "next/navigation";

import { CheckoutForm } from "@/app/checkout-form";
import { getClient, isMockMode } from "@/lib/client";
import { ApiNotReadyError } from "@/lib/errors";
import { formatMoney } from "@/lib/money";
import type { CheckoutSession } from "@/lib/types";

export default async function CheckoutPage({ params }: { params: Promise<{ sessionId: string }> }) {
  const { sessionId } = await params;
  let unavailable = "";
  let session: CheckoutSession | null = null;
  try {
    session = await getClient().getSession(sessionId);
  } catch (error) {
    if (error instanceof ApiNotReadyError) {
      unavailable = error.message;
    } else {
      throw error;
    }
  }
  if (unavailable) {
    return (
      <main>
        <p role="alert">{unavailable}</p>
      </main>
    );
  }
  if (!session) {
    notFound();
  }
  return (
    <main>
      <section className="panel stack">
        <div>
          <h1>結帳</h1>
          <p className="amount" data-testid="amount">
            {formatMoney(session.amount, session.currency)}
          </p>
          <p className="minor" data-testid="amount-minor">
            {session.amount} {session.currency}
          </p>
        </div>
        <CheckoutForm session={session} mockMode={isMockMode()} />
      </section>
    </main>
  );
}
