import { useEffect, useState } from "react";
import { subscribeToLiveEvents, type LiveEvent } from "../services/sse";

/**
 * Keeps the newest N live events in component state.
 *
 * The effect returns the unsubscribe function from subscribeToLiveEvents —
 * without that return, every mount would leave another interval running.
 */
export function useLiveEvents(max = 20): LiveEvent[] {
  const [events, setEvents] = useState<LiveEvent[]>([]);

  useEffect(() => {
    const unsubscribe = subscribeToLiveEvents((event) => {
      setEvents((current) => [event, ...current].slice(0, max));
    });
    return unsubscribe;
  }, [max]);

  return events;
}
