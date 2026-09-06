import { useEffect, useRef, useState } from "react";
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
const EMPTY_HEAT = { type: "FeatureCollection", features: [] } as GeoJSON.FeatureCollection;

export function MapView() {
  const holder = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const [showHeat, setShowHeat] = useState(false);
  const [showLandmarks, setShowLandmarks] = useState(false);
  const [showPois, setShowPois] = useState(false);

  const terr = useQuery({ queryKey: ["territories"], queryFn: () => api.territories() });
  const heat = useQuery({ queryKey: ["heatmap"], queryFn: () => api.heatmap(), enabled: showHeat });
  const lmk = useQuery({ queryKey: ["landmarks"], queryFn: () => api.landmarksProgress(), enabled: showLandmarks });
  const poi = useQuery({ queryKey: ["amenities"], queryFn: () => api.amenities(), enabled: showPois });
  const weather = useQuery({ queryKey: ["weather"], queryFn: () => api.weather() });
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
      m.addSource("heatmap", { type: "geojson", data: EMPTY_HEAT });
      m.addLayer({
        id: "heatmap-layer",
        type: "heatmap",
        source: "heatmap",
        layout: { visibility: "none" },
        paint: {
          "heatmap-weight": ["get", "w"],
          "heatmap-radius": 18,
          "heatmap-intensity": 1,
          "heatmap-opacity": 0.75,
        },
      });
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

  useEffect(() => {
    const m = map.current;
    if (!m || !m.getLayer("heatmap-layer")) return;
    m.setLayoutProperty("heatmap-layer", "visibility", showHeat ? "visible" : "none");
    const src = m.getSource("heatmap") as maplibregl.GeoJSONSource | undefined;
    if (src && heat.data) src.setData(heat.data);
  }, [showHeat, heat.data]);

  // Markers ref for landmarks & POIs
  const markersRef = useRef<maplibregl.Marker[]>([]);

  useEffect(() => {
    const m = map.current;
    if (!m) return;

    // Clear existing markers
    markersRef.current.forEach((marker) => marker.remove());
    markersRef.current = [];

    if (showLandmarks && lmk.data) {
      lmk.data.landmarks.forEach((l) => {
        const el = document.createElement("div");
        el.className = "landmark-marker";
        el.style.background = l.checked_in ? "var(--good)" : "var(--coral)";
        el.style.color = "#fff";
        el.style.borderRadius = "50%";
        el.style.width = "24px";
        el.style.height = "24px";
        el.style.display = "flex";
        el.style.alignItems = "center";
        el.style.justifyContent = "center";
        el.style.fontSize = "12px";
        el.style.boxShadow = "0 2px 6px rgba(0,0,0,0.3)";
        el.innerText = l.checked_in ? "★" : "📍";
        el.title = `${l.name} (${l.checked_in ? "Conquistado" : "Pendente"})`;

        const marker = new maplibregl.Marker({ element: el })
          .setLngLat([l.lng, l.lat])
          .setPopup(new maplibregl.Popup({ offset: 15 }).setHTML(`<strong>${l.name}</strong><br/>${l.blurb || ""}`))
          .addTo(m);

        markersRef.current.push(marker);
      });
    }

    if (showPois && poi.data) {
      poi.data.amenities.forEach((p) => {
        const el = document.createElement("div");
        el.className = "poi-marker";
        el.style.background = p.category === "bebedouro" ? "var(--teal)" : "var(--sun)";
        el.style.color = "#fff";
        el.style.borderRadius = "4px";
        el.style.padding = "2px 6px";
        el.style.fontSize = "11px";
        el.style.fontWeight = "bold";
        el.style.boxShadow = "0 2px 6px rgba(0,0,0,0.3)";
        el.innerText = p.category === "bebedouro" ? "💧" : "🚾";
        el.title = p.name;

        const marker = new maplibregl.Marker({ element: el })
          .setLngLat([p.lng, p.lat])
          .setPopup(new maplibregl.Popup({ offset: 15 }).setHTML(`<strong>${p.name}</strong><br/>${p.note || ""}`))
          .addTo(m);

        markersRef.current.push(marker);
      });
    }
  }, [showLandmarks, showPois, lmk.data, poi.data]);

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
        <button
          className={`btn sm ${showHeat ? "primary" : "ghost"}`}
          onClick={() => setShowHeat((v) => !v)}
        >
          Calor {showHeat ? "on" : "off"}
        </button>
        <button
          className={`btn sm ${showLandmarks ? "primary" : "ghost"}`}
          onClick={() => setShowLandmarks((v) => !v)}
        >
          Marcos {showLandmarks ? "on" : "off"}
        </button>
        <button
          className={`btn sm ${showPois ? "primary" : "ghost"}`}
          onClick={() => setShowPois((v) => !v)}
        >
          Apoio 💧🚾 {showPois ? "on" : "off"}
        </button>

        {weather.data && (
          <div style={{ marginTop: "0.5rem", padding: "0.6rem 0.8rem", background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "8px", fontSize: "0.8rem" }}>
            <div style={{ fontWeight: 700, color: "var(--teal)" }}>☀️ {weather.data.city} · {weather.data.temp_c}°C</div>
            <div style={{ color: "var(--ink-soft)" }}>Sensação {weather.data.feels_like_c}°C · UV {weather.data.uv_index} · Vento {weather.data.wind_kmh} km/h</div>
          </div>
        )}

        {showHeat && heat.isLoading && <span className="muted small">carregando calor…</span>}
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
