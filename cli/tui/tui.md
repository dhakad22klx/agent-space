# tui

`Output` is the one place printed bytes pass through. `Reader` reads a line back
and draws the command palette under it.

Raw mode turns off the terminal's echo, so `Reader` draws the input line itself
on every keystroke.

## Four invariants

**Raw mode lasts exactly one `ReadLine`** — entered on the way in, restored by a
`defer`. That is why `Output` can keep appending a plain `"\n"`: every other
message is printed between prompts, in cooked mode. Widen that window and they
all stair-step.

**One frame, one write.** That is what stops the flicker.

```
\r\x1b[J        erase from the cursor to the end of the screen
<prompt><line>
\r\n <row>      per palette row
\x1b[<n>A  \r  \x1b[<col>C     back to just past the last character typed
```

**Relative cursor moves only, never `\x1b7`/`\x1b8`.** Save/restore stores an
absolute row; drawing the palette near the bottom scrolls the screen, and the
saved row is then no longer the input line.

**The palette is a function of the line**, recomputed on every edit through the
`Suggest` hook — not a mode. So a slash mid-line, backspacing over the leading
slash, and a filter that stops matching all close it with no code of their own.

## Traps worth knowing

- `"\n"` alone drops a line without returning the carriage: raw mode cleared
  `OPOST`, so nothing translates it any more.
- `\x1b[0A` moves **one** row — a sequence with no number in it means one. Both
  moves are skipped at zero.
- Measure the cursor column on the *unpainted* string; color escapes are bytes
  that occupy no cells.
- Rows are truncated to `width - 1`. A wrapped row takes two physical lines and
  the `\x1b[nA` count is then short.
- Arrows arrive as `\x1b[A` *and* `\x1bOA` (application-keypad mode, which is
  what tmux commonly leaves terminals in). CSI needs its small grammar too, or
  ctrl-up (`\x1b[1;5A`) types `1;5` into the line.
- `Suggest` is a hook, not an interface, because `cli` imports `tui`.

## Falling back

Raw mode is Linux-only (`raw.mode.other.go` returns an error by design), and
`NewReader` requires both stdin and stdout to be terminals — stdout because a
redirect should collect text, not cursor movements. Anything else takes the
plain `ReadString` path, palette and all.

## Known limits

A line wider than the terminal wraps and the return move comes up short; CJK and
combining characters count as one cell and drift the column; up/down belong to
the palette, so there is no line history yet. No escape-to-dismiss — escape is
also the first byte of every arrow sequence, and a blocking read cannot tell
them apart without a timeout that misbehaves over ssh; Ctrl-C clears the line
instead, and leaves once the line is empty.

Ctrl-C is handled in the key loop rather than left to a signal on purpose:
`rawMode` clears `ISIG`, because a SIGINT would kill the process with the
terminal still raw, skipping the deferred restore and leaving the shell with no
echo.
