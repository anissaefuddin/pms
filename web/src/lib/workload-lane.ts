// Batch 4 (W2, W4): pure rules for creating a Task by dragging across a person's row in Workload.

/** W4: a drag allocates this many minutes on each working day in the range. */
export const laneMinutesPerDay = 60;

type Lane = { capacity: number[]; planned: number[]; booked: number[] };

/** W4: the working days (capacity > 0) between two column indexes, inclusive, in either drag direction. Weekends and holidays are skipped. */
export function laneWorkingDays(capacity: number[], a: number, b: number): number[] {
  const days: number[] = [];
  for (let i = Math.min(a, b); i <= Math.max(a, b); i++) if (capacity[i] > 0) days.push(i);
  return days;
}

/** W2: days where adding the lane would go over capacity. A warning only: the Task is still created. */
export function laneOverCapacity(person: Lane, days: number[], minutes = laneMinutesPerDay): number[] {
  return days.filter((i) => person.planned[i] + person.booked[i] + minutes > person.capacity[i]);
}

/** The POST /items body. W1: the allocation is stored with the Task, in the same request. */
export function laneCreateBody(input: { title: string; spaceId: string; personId: string; startDay: string; dueDay: string }) {
  return {
    type: "task",
    title: input.title.trim(),
    locations: [{ space_id: input.spaceId }],
    start_date: input.startDay,
    due_date: input.dueDay,
    assignee_ids: [input.personId],
    assignee_allocations: [{ assignee_id: input.personId, allocation: { unit: "minutes_per_day", value: laneMinutesPerDay } }],
  };
}
