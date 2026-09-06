import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { api, type FeatureCollection } from "../lib/api";
import { area } from "../lib/format";

const FORTALEZA: [number, number] = [-38.523, -3.731];
// Estilo demo do MapLibre (OSM, sem chave). Trocar pelo style JSON próprio
// (MapTiler/Protomaps) quando a chave de produção estiver configurada.
const STYLE = "https://demotiles.maplibre.org/style.json";
const RUNNER = "#08a6a0";
const EMPTY = { type: "FeatureCollection", features: [] } as FeatureCollection;

export function MapView() {
  const holder = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);

  const terr = useQuery({ queryKey: ["territories"], queryFn: () => api.territories() });
  const fc = terr.data ?? EMPTY;

  useEffect(() => {
    if (!holder.current || map.current) return;
    const m = new maplibregl.Map({
      container: holder.current,
      style: STYLE,
      center: FORTALEZA,
      zoom: 12,
      attributionControl: { compact: true },
    });
    m.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
    m.on("load", () => {
      m.addSource("territories", { type: "geojson", data: EMPTY });
      m.addLayer({
        id: "territories-fill",
        type: "fill",
        source: "territories",
        paint: { "fill-color": RUNNER, "fill-opacity": 0.22 },
      });
      m.addLayer({
        id: "territories-line",
        type: "line",
        source: "territories",
        paint: { "line-color": RUNNER, "line-width": 2.5 },
      });
      map.current = m;
      pushData(m, fc);
    });
    return () => {
      m.remove();
      map.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (map.current) pushData(map.current, fc);
  }, [fc]);

  const count = fc.features.length;
  const total = fc.features.reduce((s, f) => s + (f.properties?.area_m2 ?? 0), 0);

  return (
    <div className="map-page">
      <div ref={holder} className="map" />
      <div className="map-overlay">
        <div className="chip-stat">
          <span className="v">{count}</span>
          <span className="l">quarteirões</span>
        </div>
        <div className="chip-stat">
          <span className="v">{area(total)}</span>
          <span className="l">cobertos</span>
        </div>
        {terr.isLoading && <span className="muted small">carregando território…</span>}
        {terr.isError && <span className="err small">falha ao carregar território</span>}
        {!terr.isLoading && count === 0 && (
          <span className="muted small">
            Seu mapa está em branco. Corra em circuito para pintar o primeiro quarteirão.
          </span>
        )}
      </div>
    </div>
  );
}

function pushData(m: maplibregl.Map, fc: FeatureCollection) {
  const src = m.getSource("territories") as maplibregl.GeoJSONSource | undefined;
  if (!src) return;
  src.setData(fc);
  if (fc.features.length > 0) {
    const b = new maplibregl.LngLatBounds();
    for (const f of fc.features) {
      if ("coordinates" in f.geometry) walk(f.geometry.coordinates, (lng, lat) => b.extend([lng, lat]));
    }
    if (!b.isEmpty()) m.fitBounds(b, { padding: 80, maxZoom: 15, duration: 600 });
  }
}

function walk(coords: unknown, fn: (lng: number, lat: number) => void) {
  if (!Array.isArray(coords)) return;
  if (typeof coords[0] === "number" && typeof coords[1] === "number") {
    fn(coords[0], coords[1]);
    return;
  }
  for (const c of coords) walk(c, fn);
}
