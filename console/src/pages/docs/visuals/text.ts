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
 * A token that is punctuation only — the `·` these labels use to separate a
 * language from a port from a caveat. It is a break opportunity, never a word.
 */
const SEPARATOR = /^[^\p{L}\p{N}]+$/u;

/**
 * Drop any separator left stranded at the end of a line. The line break itself
 * already separates the two halves, so a trailing `·` is noise — and a leading
 * one on the next line is worse. At least one token always survives.
 */
function withoutTrailingSeparator(line: string): string {
  const tokens = line.split(" ");
  while (tokens.length > 1 && SEPARATOR.test(tokens[tokens.length - 1])) tokens.pop();
  return tokens.join(" ");
}

/**
 * Greedy word wrap to `maxChars` per line.
 *
 * A single word longer than `maxChars` is left whole on its own line rather
 * than broken: a port range, an `agos-` prefix, or a model id has to stay
 * readable, and every such word in these visuals still fits its box.
 *
 * Separators are handled specially, so "Anthropic · OpenAI · Ollama" wraps to
 * "Anthropic" / "OpenAI · Ollama" rather than dangling the dot at the end of
 * the first line. No line may begin or end with one.
 */
export function wrapMono(text: string, maxChars: number): string[] {
  const lines: string[] = [];
  let line = "";
  for (const word of text.split(" ")) {
    if (word === "") continue;
    // A separator can never open a line: if one lands here, the break it would
    // have followed is doing its job already.
    if (line === "" && SEPARATOR.test(word)) continue;
    const candidate = line === "" ? word : `${line} ${word}`;
    if (line === "" || candidate.length <= maxChars) {
      line = candidate;
    } else {
      lines.push(withoutTrailingSeparator(line));
      line = SEPARATOR.test(word) ? "" : word;
    }
  }
  if (line !== "") lines.push(withoutTrailingSeparator(line));
  return lines;
}
