import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { api, type FeatureCollection } from "../lib/api";
import { area } from "../lib/format";

const FORTALEZA: [number, number] = [-38.523, -3.731];

// Base raster sem chave: OSM (ruas) + Esri World Imagery (satélite). Provisório
// até entrar o style vetorial próprio (MapTiler/Protomaps — plano §7).
const STYLE: maplibregl.StyleSpecification = {
  version: 8,
  glyphs: "https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf",
  sources: {
    osm: {
      type: "raster",
      tiles: ["https://a.tile.openstreetmap.org/{z}/{x}/{y}.png"],
      tileSize: 256,
      maxzoom: 19,
      attribution: "© OpenStreetMap",
    },
    sat: {
      type: "raster",
      tiles: [
        "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
      ],
      tileSize: 256,
      maxzoom: 19,
      attribution: "Esri, Maxar, Earthstar Geographics",
    },
  },
  layers: [
    { id: "osm", type: "raster", source: "osm" },
    { id: "sat", type: "raster", source: "sat", layout: { visibility: "none" } },
  ],
};

const RUNNER = "#08a6a0";
const EMPTY = { type: "FeatureCollection", features: [] } as FeatureCollection;
const EMPTY_GJ = { type: "FeatureCollection", features: [] } as GeoJSON.FeatureCollection;

// Backend em Go serializa slice vazio como `null`. Normaliza qualquer
// FeatureCollection para ter `features: []` antes de ir pro maplibre.
function fcSafe<T extends { features?: unknown }>(fc: T | undefined | null): T {
  if (!fc) return { type: "FeatureCollection", features: [] } as unknown as T;
  if (!Array.isArray(fc.features)) return { ...fc, features: [] };
  return fc;
}

type LayerKey = "heat" | "landmarks" | "routes" | "pois" | "risk" | "hazards";

export function MapView() {
  const holder = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const ready = useRef(false);

  const [collapsed, setCollapsed] = useState(false);
  const [base, setBase] = useState<"osm" | "sat">("osm");
  const [scope, setScope] = useState<"me" | "friends">("me");
  const [heatScope, setHeatScope] = useState<"me" | "friends" | "city">("me");
  const [on, setOn] = useState<Record<LayerKey, boolean>>({
    heat: true, landmarks: true, routes: true, pois: true, risk: true, hazards: true,
  });
  const toggle = (k: LayerKey) => setOn((s) => ({ ...s, [k]: !s[k] }));

  const terr = useQuery({ queryKey: ["territories", scope], queryFn: () => api.territories(scope) });
  const heat = useQuery({ queryKey: ["heatmap", heatScope], queryFn: () => api.heatmap(heatScope), enabled: on.heat });
  const lmk = useQuery({ queryKey: ["landmarks"], queryFn: () => api.landmarksProgress(), enabled: on.landmarks });
  const poi = useQuery({ queryKey: ["amenities"], queryFn: () => api.amenities(), enabled: on.pois });
  const routes = useQuery({ queryKey: ["routesGeo"], queryFn: () => api.routes(), enabled: on.routes });
  const risk = useQuery({ queryKey: ["riskZones"], queryFn: () => api.riskZones(), enabled: on.risk });
  const haz = useQuery({ queryKey: ["hazards"], queryFn: () => api.hazards(), enabled: on.hazards });
  const weather = useQuery({ queryKey: ["weather"], queryFn: () => api.weather() });

  const fc = fcSafe(terr.data ?? EMPTY);

  // --- init ---
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

    const geo = new maplibregl.GeolocateControl({
      positionOptions: { enableHighAccuracy: true },
      trackUserLocation: true,
      showAccuracyCircle: true,
    });
    m.addControl(geo, "top-right");

    m.on("load", () => {
      m.addSource("risk", { type: "geojson", data: EMPTY_GJ });
      m.addLayer({ id: "risk-fill", type: "fill", source: "risk", layout: { visibility: "none" }, paint: { "fill-color": "#c7402b", "fill-opacity": 0.16 } });
      m.addLayer({ id: "risk-line", type: "line", source: "risk", layout: { visibility: "none" }, paint: { "line-color": "#c7402b", "line-width": 1.5, "line-dasharray": [2, 2] } });

      m.addSource("routes", { type: "geojson", data: EMPTY_GJ });
      m.addLayer({ id: "routes-line", type: "line", source: "routes", layout: { visibility: "none", "line-cap": "round" }, paint: { "line-color": "#6E5AA6", "line-width": 3 } });

      m.addSource("heatmap", { type: "geojson", data: EMPTY_GJ });
      m.addLayer({
        id: "heatmap-layer", type: "heatmap", source: "heatmap", layout: { visibility: "none" },
        paint: { "heatmap-weight": ["coalesce", ["get", "w"], 1], "heatmap-radius": 20, "heatmap-opacity": 0.75 },
      });

      m.addSource("territories", { type: "geojson", data: EMPTY });
      m.addLayer({ id: "territories-fill", type: "fill", source: "territories", paint: { "fill-color": ["coalesce", ["get", "color_hex"], RUNNER], "fill-opacity": 0.22 } });
      m.addLayer({ id: "territories-line", type: "line", source: "territories", paint: { "line-color": ["coalesce", ["get", "color_hex"], RUNNER], "line-width": 2.5 } });

      m.addSource("hazards", { type: "geojson", data: EMPTY_GJ });
      m.addLayer({
        id: "hazards-pt", type: "circle", source: "hazards", layout: { visibility: "none" },
        paint: { "circle-radius": 7, "circle-color": "#C98F2E", "circle-stroke-color": "#fff", "circle-stroke-width": 2 },
      });

      map.current = m;
      ready.current = true;
      pushTerr(m, fc);
      // já pede a permissão de GPS e centra no usuário
      setTimeout(() => geo.trigger(), 400);
    });

    return () => {
      m.remove();
      map.current = null;
      ready.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // --- base ---
  useEffect(() => {
    const m = map.current;
    if (!m || !ready.current) return;
    m.setLayoutProperty("osm", "visibility", base === "osm" ? "visible" : "none");
    m.setLayoutProperty("sat", "visibility", base === "sat" ? "visible" : "none");
  }, [base]);

  // --- território ---
  useEffect(() => {
    if (map.current && ready.current) pushTerr(map.current, fc);
  }, [fc]);

  // --- camadas GeoJSON: visibilidade + dados ---
  useEffect(() => {
    const m = map.current;
    if (!m || !ready.current) return;
    const vis = (ids: string[], show: boolean) =>
      ids.forEach((id) => m.getLayer(id) && m.setLayoutProperty(id, "visibility", show ? "visible" : "none"));
    const setData = (src: string, data: GeoJSON.FeatureCollection) => {
      const s = m.getSource(src) as maplibregl.GeoJSONSource | undefined;
      if (s) s.setData(data);
    };

    vis(["heatmap-layer"], on.heat);
    vis(["risk-fill", "risk-line"], on.risk);
    vis(["routes-line"], on.routes);
    vis(["hazards-pt"], on.hazards);

    if (heat.data) setData("heatmap", fcSafe(heat.data));
    if (risk.data) setData("risk", fcSafe(risk.data));
    if (haz.data) {
      setData("hazards", {
        type: "FeatureCollection",
        features: (haz.data.hazards ?? []).map((h) => ({
          type: "Feature",
          geometry: { type: "Point", coordinates: [h.lng, h.lat] },
          properties: { type: h.type, note: h.note ?? "" },
        })),
      });
    }
    if (routes.data) {
      setData("routes", {
        type: "FeatureCollection",
        features: (routes.data.routes ?? [])
          .filter((r) => !!r.geojson)
          .map((r) => ({ type: "Feature", geometry: JSON.parse(r.geojson), properties: { name: r.name } })),
      });
    }
  }, [on, heat.data, risk.data, haz.data, routes.data]);

  // --- markers (marcos + POIs) ---
  const markersRef = useRef<maplibregl.Marker[]>([]);
  useEffect(() => {
    const m = map.current;
    if (!m || !ready.current) return;
    markersRef.current.forEach((mk) => mk.remove());
    markersRef.current = [];

    if (on.landmarks && lmk.data) {
      (lmk.data.landmarks ?? []).forEach((l) => {
        const el = pin(l.checked_in ? "★" : "📍", l.checked_in ? "var(--good)" : "var(--coral)");
        markersRef.current.push(
          new maplibregl.Marker({ element: el })
            .setLngLat([l.lng, l.lat])
            .setPopup(new maplibregl.Popup({ offset: 15 }).setHTML(`<strong>${esc(l.name)}</strong><br/>${esc(l.blurb || "")}`))
            .addTo(m),
        );
      });
    }
    if (on.pois && poi.data) {
      (poi.data.amenities ?? []).forEach((p) => {
        const el = pin(p.category === "bebedouro" ? "💧" : "🚾", "var(--teal)");
        markersRef.current.push(
          new maplibregl.Marker({ element: el })
            .setLngLat([p.lng, p.lat])
            .setPopup(new maplibregl.Popup({ offset: 15 }).setHTML(`<strong>${esc(p.name)}</strong><br/>${esc(p.note || "")}`))
            .addTo(m),
        );
      });
    }
  }, [on.landmarks, on.pois, lmk.data, poi.data]);

  const count = fc.features.length;
  const totalArea = fc.features.reduce((s, f) => s + (f.properties?.area_m2 ?? 0), 0);

  return (
    <div className="map-page">
      <div ref={holder} className="map" />

      {collapsed ? (
        <button className="map-fab" onClick={() => setCollapsed(false)} aria-label="Abrir camadas">
          ▨
        </button>
      ) : (
        <div className="map-overlay">
          <div className="ov-head">
            <div>
              <strong>{count}</strong> <span className="muted small">quart.</span>{" "}
              <strong>{area(totalArea)}</strong>
            </div>
            <button className="ov-min" onClick={() => setCollapsed(true)} aria-label="Minimizar">
              –
            </button>
          </div>

          <div className="ov-sec">Base</div>
          <div className="ov-row">
            <button className={`btn sm ${base === "osm" ? "primary" : "ghost"}`} onClick={() => setBase("osm")}>Mapa</button>
            <button className={`btn sm ${base === "sat" ? "primary" : "ghost"}`} onClick={() => setBase("sat")}>Satélite</button>
          </div>

          <div className="ov-sec">Território</div>
          <div className="ov-row">
            <button className={`btn sm ${scope === "me" ? "primary" : "ghost"}`} onClick={() => setScope("me")}>Meu</button>
            <button className={`btn sm ${scope === "friends" ? "primary" : "ghost"}`} onClick={() => setScope("friends")}>Amigos</button>
          </div>

          <div className="ov-sec">Camadas</div>
          <div className="ov-layers">
            <LayerBtn label="Mapa de calor" active={on.heat} onClick={() => toggle("heat")} />
            <LayerBtn label="Marcos" active={on.landmarks} onClick={() => toggle("landmarks")} />
            <LayerBtn label="Rotas" active={on.routes} onClick={() => toggle("routes")} />
            <LayerBtn label="Bebedouros / banheiros" active={on.pois} onClick={() => toggle("pois")} />
            <LayerBtn label="Zonas de risco" active={on.risk} onClick={() => toggle("risk")} />
            <LayerBtn label="Alertas na via" active={on.hazards} onClick={() => toggle("hazards")} />
          </div>

          {on.heat && (
            <div className="ov-row">
              {(["me", "friends", "city"] as const).map((s) => (
                <button key={s} className={`btn sm ${heatScope === s ? "primary" : "ghost"}`} onClick={() => setHeatScope(s)}>
                  {s === "me" ? "eu" : s === "friends" ? "rede" : "cidade"}
                </button>
              ))}
            </div>
          )}

          {weather.data && (
            <div className="ov-weather">
              <strong>{weather.data.city} · {weather.data.temp_c}°C</strong>
              <span className="muted">Sensação {weather.data.feels_like_c}° · UV {weather.data.uv_index} · Vento {weather.data.wind_kmh} km/h</span>
            </div>
          )}

          {terr.isError && <span className="err small">falha ao carregar território</span>}
          {!terr.isLoading && count === 0 && (
            <span className="muted small">Mapa em branco — corra em circuito para pintar o primeiro quarteirão.</span>
          )}
        </div>
      )}
    </div>
  );
}

function LayerBtn({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button className={`ov-layer${active ? " on" : ""}`} onClick={onClick}>
      <span className="dot" />
      {label}
    </button>
  );
}

function pin(glyph: string, color: string) {
  const el = document.createElement("div");
  Object.assign(el.style, {
    background: color, color: "#fff", borderRadius: "50%", width: "26px", height: "26px",
    display: "flex", alignItems: "center", justifyContent: "center", fontSize: "13px",
    boxShadow: "0 2px 6px rgba(0,0,0,0.35)", cursor: "pointer",
  });
  el.textContent = glyph;
  return el;
}

function esc(s: string) {
  return s.replace(/[<>&"]/g, (c) => ({ "<": "&lt;", ">": "&gt;", "&": "&amp;", '"': "&quot;" })[c]!);
}

function pushTerr(m: maplibregl.Map, fc: FeatureCollection) {
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
