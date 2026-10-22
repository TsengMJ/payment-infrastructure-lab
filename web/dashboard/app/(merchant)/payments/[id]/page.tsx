import { getClient } from "@/lib/client";
import { NotFoundError } from "@/lib/errors";
import type { TimelineEvent } from "@/lib/types";

export default async function TimelinePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  let missing = "";
  let events: TimelineEvent[] = [];
  try {
    events = await getClient().getTimeline(id);
  } catch (error) {
    if (error instanceof NotFoundError) {
      missing = error.message;
    } else {
      throw error;
    }
  }
  if (missing) {
    return (
      <section>
        <h1>交易時間線</h1>
        <p role="alert">{missing}</p>
      </section>
    );
  }
  return (
    <section>
      <h1>交易時間線</h1>
      <p data-testid="payment-id">{id}</p>
      <ol className="timeline">
        {events.map((event) => (
          <li key={event.id}>
            <strong>{event.title}</strong>
            <div>{event.detail}</div>
            <time dateTime={event.at}>{event.at}</time>
          </li>
        ))}
      </ol>
    </section>
  );
}
