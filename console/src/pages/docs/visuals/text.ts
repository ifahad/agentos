/**
 * Text fitting for the docs visuals.
 *
 * Every string these illustrations draw is set in the mono face, where each
 * glyph advances exactly 0.6em. That makes a character count an exact width
 * rather than a guess, so the visuals can wrap and centre their own labels
 * without a DOM-measuring pass, a layout effect, or a dependency.
 *
 * This matters because the geometry module owns the boxes and the content owns
 * the strings, and several of those strings are far wider than the box they sit
 * in: "Anthropic · OpenAI · Ollama" is 162 units of text in a 128-unit box, and
 * "connector or sandbox, constrained in-process" is 264 units of text in a
 * 160-unit column. Drawing them on one line would push them over their own
 * borders and, in two cases, straight through an edge or out of the viewBox.
 */

/** Advance width of one glyph in the mono face, as a fraction of the font size. */
export const MONO_ADVANCE = 0.6;

/** How many mono characters fit in `width` user units at `fontSize`. */
export function monoCharsInWidth(width: number, fontSize: number): number {
  return Math.max(1, Math.floor(width / (fontSize * MONO_ADVANCE)));
}

/**
 * Greedy word wrap to `maxChars` per line.
 *
 * A single word longer than `maxChars` is left whole on its own line rather
 * than broken: a port range, an `agos-` prefix, or a model id has to stay
 * readable, and every such word in these visuals still fits its box.
 */
export function wrapMono(text: string, maxChars: number): string[] {
  const lines: string[] = [];
  let line = "";
  for (const word of text.split(" ")) {
    if (word === "") continue;
    const candidate = line === "" ? word : `${line} ${word}`;
    if (line === "" || candidate.length <= maxChars) {
      line = candidate;
    } else {
      lines.push(line);
      line = word;
    }
  }
  if (line !== "") lines.push(line);
  return lines;
}
