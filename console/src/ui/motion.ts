/**
 * Shared motion presets — one easing curve, two durations, consistent
 * stagger. All framer-motion animation in the console should import from
 * here so motion stays telemetry, not decoration.
 *
 * Reduced motion: components gate JS-driven animation with framer-motion's
 * `useReducedMotion()` (or the `reduced` variants below); CSS keyframes and
 * transitions are disabled globally in styles.css.
 */
import type { Variants, Transition } from "framer-motion";

/**
 * Seconds. Hovers, presses, focus.
 * NOTE: does not match CSS --dur-fast (110ms, styles.css) — the JS and CSS
 * motion tokens drifted out of sync when they were written. Not corrected
 * here: this is a refactor (route call sites through the shared presets),
 * and reconciling the two is a design decision, not a consolidation.
 */
export const DUR_FAST = 0.12;
/**
 * Seconds. Entrances, transitions.
 * NOTE: does not match CSS --dur-med (180ms, styles.css) — same pre-existing
 * drift as DUR_FAST above.
 */
export const DUR_MED = 0.2;
/** Page transition duration (spec: 160ms fade + 6px rise). */
export const DUR_PAGE = 0.16;
/** cubic-bezier(0.2, 0.8, 0.2, 1) — matches --ease. */
export const EASE: [number, number, number, number] = [0.2, 0.8, 0.2, 1];

/** Per-item stagger for lists/rows (spec: 24–40ms, cap ~6 visible). */
export const STAGGER = 0.032;
/** Cap on how many items animate in a staggered entrance. */
export const STAGGER_MAX_ITEMS = 6;

export const transition: Transition = { duration: DUR_MED, ease: EASE };
export const transitionFast: Transition = { duration: DUR_FAST, ease: EASE };

/** Fade + 6px rise entrance. Used for cards, panels, page content. */
export const fadeRise: Variants = {
  hidden: { opacity: 0, y: 6 },
  show: { opacity: 1, y: 0, transition },
  exit: { opacity: 0, y: 4, transition: transitionFast },
};

/** Pure fade (no movement) — panels arriving after stats, table bodies. */
export const fade: Variants = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition },
  exit: { opacity: 0, transition: transitionFast },
};

/** Stagger container — pair with `staggerItem` children. */
export const staggerContainer: Variants = {
  hidden: {},
  show: {
    transition: { staggerChildren: STAGGER, delayChildren: 0.02 },
  },
};

/** Child of `staggerContainer` (table rows, list items, feed entries). */
export const staggerItem: Variants = {
  hidden: { opacity: 0, y: 4 },
  show: { opacity: 1, y: 0, transition: { duration: DUR_MED, ease: EASE } },
};

/** Reduced-motion-safe variants: movement removed, fade kept instant-ish. */
export const fadeRiseReduced: Variants = {
  hidden: { opacity: 1, y: 0 },
  show: { opacity: 1, y: 0 },
  exit: { opacity: 1, y: 0 },
};

export const staggerItemReduced: Variants = {
  hidden: { opacity: 1, y: 0 },
  show: { opacity: 1, y: 0 },
};

/** Modal — backdrop fade + panel scale/rise. */
export const modalBackdrop: Variants = {
  hidden: { opacity: 0 },
  show: { opacity: 1, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, transition: { duration: DUR_FAST, ease: EASE } },
};

export const modalPanel: Variants = {
  hidden: { opacity: 0, scale: 0.97, y: 8 },
  show: { opacity: 1, scale: 1, y: 0, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, scale: 0.98, y: 4, transition: { duration: DUR_FAST, ease: EASE } },
};

export const modalPanelReduced: Variants = {
  hidden: { opacity: 1, scale: 1, y: 0 },
  show: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 1, scale: 1, y: 0 },
};

/** Toast — slide in from the right + fade (bottom-right stack). */
export const toastItem: Variants = {
  hidden: { opacity: 0, x: 24 },
  show: { opacity: 1, x: 0, transition: { duration: DUR_MED, ease: EASE } },
  exit: { opacity: 0, x: 16, transition: { duration: DUR_FAST, ease: EASE } },
};

export const toastItemReduced: Variants = {
  hidden: { opacity: 1, x: 0 },
  show: { opacity: 1, x: 0 },
  exit: { opacity: 0, transition: { duration: 0.01 } },
};

/** Page transition on route change — 160ms fade + 6px rise. */
export const pageTransition: Variants = {
  hidden: { opacity: 0, y: 6 },
  show: { opacity: 1, y: 0, transition: { duration: DUR_PAGE, ease: EASE } },
  exit: { opacity: 0, transition: { duration: DUR_FAST, ease: EASE } },
};
