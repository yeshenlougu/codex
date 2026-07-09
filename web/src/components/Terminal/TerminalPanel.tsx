import { useEffect, useRef } from 'react';

export default function TerminalPanel() {
  const ref = useRef<HTMLDivElement>(null);
  const termRef = useRef<any>(null);

  useEffect(() => {
    let disposed = false;

    const init = async () => {
      try {
        const { Terminal } = await import('@xterm/xterm');
        const { FitAddon } = await import('@xterm/addon-fit');
        const { WebLinksAddon } = await import('@xterm/addon-web-links');

        if (disposed || !ref.current) return;

        const term = new Terminal({
          cursorBlink: true,
          fontSize: 13,
          fontFamily: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace",
          theme: {
            background: '#0d1117',
            foreground: '#e6edf3',
            cursor: '#58a6ff',
            selectionBackground: '#58a6ff33',
            black: '#21262d',
            red: '#f85149',
            green: '#3fb950',
            yellow: '#d2991d',
            blue: '#58a6ff',
            magenta: '#bc8cff',
            cyan: '#39c5cf',
            white: '#e6edf3',
            brightBlack: '#484f58',
            brightRed: '#ff6b6b',
            brightGreen: '#56d364',
            brightYellow: '#e3b341',
            brightBlue: '#79c0ff',
            brightMagenta: '#d2a8ff',
            brightCyan: '#56d4dd',
            brightWhite: '#ffffff',
          },
        });

        const fitAddon = new FitAddon();
        const webLinksAddon = new WebLinksAddon();

        term.loadAddon(fitAddon);
        term.loadAddon(webLinksAddon);
        term.open(ref.current);
        fitAddon.fit();

        term.writeln('\x1b[1;36mCodex Go Terminal\x1b[0m');
        term.writeln('Type commands and press Enter. \x1b[90m(Not yet connected to shell)\x1b[0m');
        term.writeln('');

        // Simulated prompt
        term.write('\x1b[32m$\x1b[0m ');
        termRef.current = term;

        // Resize on window resize
        const handleResize = () => { try { fitAddon.fit(); } catch {} };
        window.addEventListener('resize', handleResize);
        return () => window.removeEventListener('resize', handleResize);
      } catch (e) {
        console.warn('xterm not available:', e);
      }
    };

    init();
    return () => { disposed = true; termRef.current?.dispose(); };
  }, []);

  return <div ref={ref} className="h-full w-full" />;
}
