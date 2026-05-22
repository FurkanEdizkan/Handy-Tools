/**
 * mascot.ts — derives the sidebar mascot's mood from the live job queue.
 *
 * The htools-gui design wants Wrenly/Hopper to react to work: watching while a
 * job runs, leaning into stressed/tired as it grinds on, a brief happy pulse
 * on completion, and a worried base mood when a failed job sits unattended.
 */
import { readable, type Readable } from 'svelte/store';
import { jobs, type Job } from './jobs';

export type MascotState =
  | 'idle'
  | 'thinking'
  | 'watching'
  | 'stressed'
  | 'tired'
  | 'happy'
  | 'worried';

export interface MascotMood {
  state: MascotState;
  speech: string;
}

const SPEECH: Record<MascotState, string> = {
  idle: 'Ready when you are.',
  thinking: 'Pulling up the tool…',
  watching: 'Running a job — watching it go.',
  stressed: 'Lots in flight — holding it together.',
  tired: 'Almost there with the last batch.',
  happy: 'All done. Files are ready.',
  worried: 'A job failed — open Jobs to see why.',
};

/** HAPPY_MS is how long the happy pulse lingers after a run completes. */
const HAPPY_MS = 2600;

function moodFor(state: MascotState): MascotMood {
  return { state, speech: SPEECH[state] };
}

/** baseState picks the resting mood from the current queue (no happy pulse). */
function baseState(list: Job[]): MascotState {
  const running = list.filter((j) => j.status === 'running');
  if (running.length > 0) {
    const p = running.reduce((sum, j) => sum + j.progress, 0) / running.length;
    if (p < 0.4) return 'watching';
    if (p < 0.78) return 'stressed';
    return 'tired';
  }
  if (list.some((j) => j.status === 'fail')) return 'worried';
  return 'idle';
}

/**
 * mascotMood is a readable {state, speech}. It tracks the running→idle edge to
 * fire a happy pulse, and otherwise reflects baseState.
 */
export const mascotMood: Readable<MascotMood> = readable<MascotMood>(moodFor('idle'), (set) => {
  let prevRunning = 0;
  let happyTimer: ReturnType<typeof setTimeout> | undefined;
  let latest: Job[] = [];

  function recompute(): void {
    set(moodFor(baseState(latest)));
  }

  const unsub = jobs.subscribe((list) => {
    latest = list;
    const running = list.filter((j) => j.status === 'running').length;
    const justFinished = prevRunning > 0 && running === 0;
    prevRunning = running;

    if (justFinished && !list.some((j) => j.status === 'fail')) {
      if (happyTimer) clearTimeout(happyTimer);
      set(moodFor('happy'));
      happyTimer = setTimeout(() => {
        happyTimer = undefined;
        recompute();
      }, HAPPY_MS);
      return;
    }
    // While the happy pulse is showing, let it linger; otherwise reflect now.
    if (!happyTimer) recompute();
  });

  return () => {
    unsub();
    if (happyTimer) clearTimeout(happyTimer);
  };
});
