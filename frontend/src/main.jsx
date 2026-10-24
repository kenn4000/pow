import React, { useEffect, useRef } from 'react';
import { createRoot } from 'react-dom/client';
import shell from 'virtual:game-shell';
import { mountGame } from 'virtual:game-controller';
import 'virtual:game-style.css';

function App() {
  const host = useRef(null);

  useEffect(() => {
    const root = host.current;
    const events = new AbortController();
    const scopedDocument = {
      getElementById: id => root.querySelector(`#${id}`),
      querySelector: selector => root.querySelector(selector),
      createElement: tag => document.createElement(tag),
      addEventListener: (type, listener) => root.addEventListener(type, listener, { signal: events.signal }),
    };
    const dispose = mountGame(scopedDocument);
    return () => {
      events.abort();
      dispose();
    };
  }, []);

  return <div ref={host} dangerouslySetInnerHTML={{ __html: shell }} />;
}

createRoot(document.getElementById('root')).render(<App />);
