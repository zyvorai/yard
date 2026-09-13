import { useCallback, useRef, useState } from "react";
import Map, { Marker, type MapLayerMouseEvent, type MapRef } from "react-map-gl/maplibre";
import "maplibre-gl/dist/maplibre-gl.css";
import { api } from "../lib/api";
import { MAP_STYLE_DARK, MAP_STYLE_LIGHT, useTheme } from "../lib/theme";

type GeocodeResult = { display_name: string; lat: number; lon: number };

const DEFAULT_CENTER: [number, number] = [75.34, 19.88];

type Props = {
  latitude?: number;
  longitude?: number;
  onChange: (lat: number, lng: number) => void;
};

export default function LocationPicker({ latitude, longitude, onChange }: Props) {
  const { theme } = useTheme();
  const style = theme === "dark" ? MAP_STYLE_DARK : MAP_STYLE_LIGHT;
  const mapRef = useRef<MapRef>(null);
  const [q, setQ] = useState("");
  const [results, setResults] = useState<GeocodeResult[]>([]);
  const [searchErr, setSearchErr] = useState("");
  const [searching, setSearching] = useState(false);

  const hasPos = latitude != null && longitude != null;
  const center: [number, number] = hasPos ? [longitude!, latitude!] : DEFAULT_CENTER;

  const place = useCallback((lat: number, lng: number, fly?: boolean) => {
    onChange(lat, lng);
    if (fly) mapRef.current?.flyTo({ center: [lng, lat], zoom: 14, duration: 600 });
  }, [onChange]);

  const onClick = useCallback((e: MapLayerMouseEvent) => {
    place(e.lngLat.lat, e.lngLat.lng);
  }, [place]);

  async function search() {
    if (!q.trim()) return;
    setSearching(true);
    setSearchErr("");
    try {
      const res = await api<{ results: GeocodeResult[] }>(`/api/v1/geocode?q=${encodeURIComponent(q.trim())}`);
      setResults(res.results || []);
      if (!res.results?.length) setSearchErr("No matches found.");
    } catch (ex) {
      setResults([]);
      setSearchErr(ex instanceof Error ? ex.message : "search failed");
    } finally {
      setSearching(false);
    }
  }

  function pick(r: GeocodeResult) {
    place(r.lat, r.lon, true);
    setResults([]);
    setQ(r.display_name);
  }

  return (
    <div className="location-picker">
      <div className="location-picker-search">
        <input
          className="search"
          placeholder="Search an address or place…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              void search();
            }
          }}
        />
        <button type="button" className="btn small ghost" disabled={searching} onClick={() => void search()}>
          {searching ? "Searching…" : "Search"}
        </button>
      </div>
      {searchErr && <p className="form-error">{searchErr}</p>}
      {results.length > 0 && (
        <ul className="location-picker-results">
          {results.map((r, i) => (
            <li key={i}>
              <button type="button" onClick={() => pick(r)}>{r.display_name}</button>
            </li>
          ))}
        </ul>
      )}
      <div className="location-picker-map">
        <Map
          ref={mapRef}
          initialViewState={{ longitude: center[0], latitude: center[1], zoom: hasPos ? 12 : 4 }}
          mapStyle={style}
          style={{ width: "100%", height: "100%" }}
          onClick={onClick}
          cursor="crosshair"
        >
          {hasPos && (
            <Marker
              longitude={longitude!}
              latitude={latitude!}
              draggable
              onDragEnd={(e) => place(e.lngLat.lat, e.lngLat.lng)}
            />
          )}
        </Map>
      </div>
      <p className="location-picker-attrib">Search © OpenStreetMap contributors</p>
    </div>
  );
}
