import { notFound } from "next/navigation";

import { startFromLink } from "@/app/actions";
import { getClient } from "@/lib/client";
import { ApiNotReadyError } from "@/lib/errors";
import { formatMoney } from "@/lib/money";
import type { PaymentLink } from "@/lib/types";

export default async function PaymentLinkPage({ params }: { params: Promise<{ linkId: string }> }) {
  const { linkId } = await params;
  let unavailable = "";
  let link: PaymentLink | null = null;
  try {
    link = await getClient().getLink(linkId);
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
  if (!link) {
    notFound();
  }
  return (
    <main>
      <section className="panel stack">
        <h1>{link.title}</h1>
        <p className="amount">{formatMoney(link.amount, link.currency)}</p>
        <p className="minor" data-testid="amount-minor">
          {link.amount} {link.currency}
        </p>
        {link.status === "used" ? <p>這個連結已經使用過</p> : null}
        {link.status === "active" ? (
          <form action={startFromLink.bind(null, link.id)}>
            <button type="submit">開始付款</button>
          </form>
        ) : null}
      </section>
    </main>
  );
}
