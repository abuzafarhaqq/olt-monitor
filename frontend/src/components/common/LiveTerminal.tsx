import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

export interface LiveTerminalHandle {
  /** Feed raw bytes; xterm handles vt100, backspaces and paging markers. */
  write: (text: string) => void;
  reset: () => void;
}

interface LiveTerminalProps {
  rows?: number;
}

/**
 * One xterm instance, used wherever device output is shown.
 *
 * `disableStdin` is the important option: this terminal is a view. Nothing
 * typed here can reach a device, because nothing typed here is sent anywhere.
 * A <pre> would be simpler and worse — the OLT speaks vt100, so carriage
 * returns, backspaces and "--More--" turn into unreadable noise without a
 * terminal emulator.
 */
export const LiveTerminal = forwardRef<LiveTerminalHandle, LiveTerminalProps>(
  function LiveTerminal({ rows = 24 }, ref) {
    const hostRef = useRef<HTMLDivElement | null>(null);
    const termRef = useRef<Terminal | null>(null);

    useEffect(() => {
      if (hostRef.current === null) return;

      const term = new Terminal({
        convertEol: true,
        cursorBlink: false,
        disableStdin: true,
        fontFamily: '"JetBrains Mono", "SFMono-Regular", Consolas, monospace',
        fontSize: 12,
        scrollback: 8000,
        theme: { background: "#0e1a2b", foreground: "#d6e2f0" },
        cols: 120,
        rows,
      });
      term.open(hostRef.current);
      termRef.current = term;

      return () => {
        term.dispose();
        termRef.current = null;
      };
    }, [rows]);

    useImperativeHandle(
      ref,
      () => ({
        write: (text: string) => termRef.current?.write(text),
        reset: () => termRef.current?.reset(),
      }),
      [],
    );

    return <div className="terminal-host" ref={hostRef} />;
  },
);
