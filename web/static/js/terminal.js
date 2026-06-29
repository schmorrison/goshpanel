(() => {
  const container = document.getElementById("terminal");
  if (!container || !window.Terminal) {
    return;
  }

  const term = new Terminal({
    cursorBlink: true,
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
    theme: {
      background: "#000000",
      foreground: "#d1fae5",
    },
  });

  const fitAddon = new FitAddon();
  term.loadAddon(fitAddon);
  term.open(container);
  fitAddon.fit();

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${protocol}//${window.location.host}/terminal/ws`);
  socket.binaryType = "arraybuffer";

  socket.onopen = () => {
    term.write("Connecting to shell...\r\n");
    sendResize();
  };

  socket.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(event.data));
      return;
    }
    term.write(event.data);
  };

  socket.onclose = () => {
    term.write("\r\n[session closed]\r\n");
  };

  term.onData((data) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(data);
    }
  });

  term.onResize(({ cols, rows }) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "resize", cols, rows }));
    }
  });

  window.addEventListener("resize", () => {
    fitAddon.fit();
    sendResize();
  });

  function sendResize() {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    }
  }
})();
