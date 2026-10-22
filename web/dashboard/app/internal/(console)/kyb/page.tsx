import { decideKybAction } from "@/app/actions";
import { getClient } from "@/lib/client";
import { currentInternal } from "@/lib/session";

export default async function KybPage({ searchParams }: { searchParams: Promise<{ error?: string }> }) {
  const user = await currentInternal();
  const params = await searchParams;
  if (!user) {
    return null;
  }
  const cases = await getClient().listKyb(user);
  return (
    <section className="stack">
      <h1>入駐審核</h1>
      {params.error ? <p role="alert">{params.error}</p> : null}
      {cases.map((item) => (
        <article key={item.id} className="card">
          <h2>{item.merchantName}</h2>
          <p>負責人：{item.maskedOwner}</p>
          <p>篩查：{item.screening}</p>
          <p data-testid="capability">{item.capability}</p>
          <p>
            <a href={item.vendorUrl} rel="noreferrer">
              {item.vendorLabel}
            </a>
          </p>
          {user.canReview && item.status === "pending" ? (
            <form action={decideKybAction} className="stack">
              <input type="hidden" name="caseId" value={item.id} />
              <div>
                <button type="submit" name="decision" value="approved">
                  核准
                </button>{" "}
                <button type="submit" name="decision" value="rejected">
                  拒絕
                </button>
              </div>
            </form>
          ) : null}
        </article>
      ))}
    </section>
  );
}
