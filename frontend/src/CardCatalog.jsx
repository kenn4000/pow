import React, { useEffect, useRef, useState } from 'react';
import { request } from './api';

const imageURL = art => `${import.meta.env.BASE_URL}${art.path}`;
const title = text => text.charAt(0).toUpperCase() + text.slice(1);
const tierLabel = tier => tier === 'med' ? 'Medium' : title(tier);

function ArtworkImage({ art, label, face }) {
  if (!art) return <span>Artwork unavailable</span>;
  return <span className="catalog-image">
    <img src={imageURL(art)} alt={label} loading="lazy" decoding="async" width={art.width} height={art.height} />
    {face && <span className="catalog-face">{face}</span>}
  </span>;
}

function ArtworkFacts({ art }) {
  return <dl className="card-facts">
    <dt>Color</dt><dd>{title(art.color)}</dd>
    <dt>Tier</dt><dd>{tierLabel(art.tier)}</dd>
    <dt>Pose</dt><dd>{title(art.pose)}</dd>
    <dt>Dimensions</dt><dd>{art.width} × {art.height} px</dd>
    <dt>File size</dt><dd>{(art.bytes / 1024).toFixed(1)} KiB</dd>
    <dt>File</dt><dd><a href={imageURL(art)} target="_blank" rel="noreferrer">{art.id}</a></dd>
  </dl>;
}

function CardDetails({ selection, artwork, panel, onClose }) {
  const { card, art } = selection;
  const variants = artwork.filter(item => item.tier === art.tier);
  return <aside id="card-details" className="card-details" aria-label="Card details" tabIndex={-1} ref={panel} onKeyDown={event => { if (event.key === 'Escape') onClose(); }}>
    <button onClick={onClose} aria-label="Close card details">Close</button>
    <h2>{card ? `${card.suit} ${card.face} (${card.id})` : `${title(art.color)} · ${tierLabel(art.tier)} · ${title(art.pose)}`}</h2>
    <ArtworkImage art={art} face={card?.face} label={card ? `${card.suit} ${card.face}` : art.id} />
    {card && <>
      <dl className="card-facts">
        <dt>Suit</dt><dd>{card.suit}</dd><dt>Face</dt><dd>{card.face}</dd>
        <dt>Beats</dt><dd>{card.beats}</dd><dt>Loses to</dt><dd>{card.losesTo}</dd>
        <dt>Base value</dt><dd>{card.face}; {card.face * 2} with suit advantage, before hero modifiers</dd>
      </dl>
      <p>Faces 1–3 use small artwork, 4–6 medium, and 7–9 large.</p>
    </>}
    <ArtworkFacts art={art} />
    <h3>Available colors and poses</h3>
    <div className="card-variants">{variants.map(item => <figure key={item.id}>
      <ArtworkImage art={item} face={card?.face} label={`${item.color} ${item.pose}`} />
      <figcaption>{title(item.color)} · {title(item.pose)}</figcaption>
    </figure>)}</div>
  </aside>;
}

export function CardCatalog() {
  const [catalog, setCatalog] = useState(null);
  const [error, setError] = useState('');
  const [mode, setMode] = useState('deck');
  const [skin, setSkin] = useState('');
  const [query, setQuery] = useState('');
  const [selection, setSelection] = useState(null);
  const panel = useRef(null);
  const trigger = useRef(null);

  async function load() {
    setError('');
    try { setCatalog(await request('admin/cards')); }
    catch (error) { setError(error.message); }
  }
  useEffect(() => { load(); }, []);
  useEffect(() => { panel.current?.focus(); }, [selection]);

  function select(event, art, card = null) {
    trigger.current = event.currentTarget;
    setSelection({ art, card });
  }
  function close() {
    setSelection(null);
    trigger.current?.focus();
  }
  function switchMode(next) { setMode(next); setSelection(null); }

  const artwork = catalog?.artwork || [];
  const skins = [...new Set(artwork.map(art => art.skin))];
  const term = query.trim().toLowerCase();
  const cards = (catalog?.cards || []).filter(card => `${card.id} ${card.suit} ${card.face} ${tierLabel(card.tier)}`.toLowerCase().includes(term));
  const filteredArt = artwork.filter(art => (!skin || art.skin === skin) && `${art.color} ${tierLabel(art.tier)} ${art.pose} ${art.id}`.toLowerCase().includes(term));

  return <main className="account-page card-catalog">
    <h1>Card contact sheet</h1>
    <p>Inspect every card in the deck and all embedded artwork. Select a thumbnail to open its detail panel.</p>
    <div className="catalog-toolbar">
      <div role="group" aria-label="Contact sheet view">
        <button aria-pressed={mode === 'deck'} onClick={() => switchMode('deck')}>Deck ({catalog?.cards.length || 0})</button>
        <button aria-pressed={mode === 'artwork'} onClick={() => switchMode('artwork')}>Artwork ({artwork.length})</button>
      </div>
      <label>Color<select value={skin} onChange={event => { setSkin(event.target.value); setSelection(null); }}>
        <option value="">{mode === 'deck' ? 'Default suit colors' : 'All colors'}</option>
        {skins.map(value => <option key={value} value={value}>{title(value.replace('tribal_', ''))}</option>)}
      </select></label>
      <label>Search cards or artwork<input type="search" value={query} onChange={event => setQuery(event.target.value)} /></label>
    </div>
    {error && <p role="alert">{error} <button onClick={load}>Retry</button></p>}
    {!catalog && !error && <p role="status">Loading card catalog…</p>}
    {catalog && <div className={`catalog-layout ${selection ? 'has-details' : ''}`}>
      <section aria-label="Card thumbnails">
        <p role="status">{mode === 'deck' ? cards.length : filteredArt.length} items shown</p>
        <div className="contact-sheet">
          {mode === 'deck' ? cards.map(card => {
            const art = artwork.find(item => item.skin === (skin || card.defaultSkin) && item.tier === card.tier && item.pose === 'idle');
            return <button key={card.id} className="contact-tile" aria-label={`Inspect ${card.suit} ${card.face}`} aria-controls="card-details" aria-expanded={selection?.card?.id === card.id} onClick={event => select(event, art, card)}>
              <ArtworkImage art={art} face={card.face} label={`${card.suit} ${card.face}`} />
              <strong>{card.suit} {card.face}</strong><small>{card.id} · {tierLabel(card.tier)}</small>
            </button>;
          }) : filteredArt.map(art => <button key={art.id} className="contact-tile" aria-label={`Inspect ${art.color} ${art.tier} ${art.pose}`} aria-controls="card-details" aria-expanded={!selection?.card && selection?.art.id === art.id} onClick={event => select(event, art)}>
            <ArtworkImage art={art} label={`${art.color} ${art.tier} ${art.pose}`} />
            <strong>{title(art.color)} · {tierLabel(art.tier)}</strong><small>{title(art.pose)}</small>
          </button>)}
        </div>
      </section>
      {selection && <CardDetails selection={selection} artwork={artwork} panel={panel} onClose={close} />}
    </div>}
  </main>;
}
