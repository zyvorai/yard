import { useCallback, useEffect, useMemo, useRef } from "react";
import Map, { Marker, NavigationControl, type MapRef } from "react-map-gl/maplibre";
import "maplibre-gl/dist/maplibre-gl.css";
import { MAP_STYLE_DARK, MAP_STYLE_LIGHT, useTheme } from "../lib/theme";

export type MapPin = {
  id: string;
  name: string;
  kind: string;
  health?: string;
  latitude: number;
  longitude: number;
  entity: "asset" | "site";
  last_seen_at?: string;
  external_ref?: string;
};

type Props = {
  pins: MapPin[];
  selectedId?: string | null;
  onSelect: (pin: MapPin) => void;
  interactive?: boolean;
};

function markerClass(pin: MapPin, selected: boolean) {
  const health = pin.entity === "site" ? "site" : (pin.health || "unknown");
  return `map-marker ${health}${selected ? " selected" : ""}`;
}

export default function YardMap({ pins, selectedId, onSelect }: Props) {
  const { theme } = useTheme();
  const mapRef = useRef<MapRef>(null);
  const userMoved = useRef(false);
  const style = theme === "dark" ? MAP_STYLE_DARK : MAP_STYLE_LIGHT;

  const initialView = useMemo(() => {
    if (!pins.length) {
      return { longitude: -0.12, latitude: 51.5, zoom: 3 };
    }
    const lng = pins.reduce((s, p) => s + p.longitude, 0) / pins.length;
    const lat = pins.reduce((s, p) => s + p.latitude, 0) / pins.length;
    return { longitude: lng, latitude: lat, zoom: 11 };
  }, [pins]);

  const fitPins = useCallback((animate: boolean) => {
    const map = mapRef.current;
    if (!map || !pins.length) return;
    if (pins.length === 1) {
      map.flyTo({ center: [pins[0].longitude, pins[0].latitude], zoom: 14, duration: animate ? 800 : 0 });
      return;
    }
    const lats = pins.map((p) => p.latitude);
    const lngs = pins.map((p) => p.longitude);
    map.fitBounds(
      [
        [Math.min(...lngs), Math.min(...lats)],
        [Math.max(...lngs), Math.max(...lats)],
      ],
      { padding: { top: 80, bottom: 80, left: 400, right: 80 }, duration: animate ? 700 : 0, maxZoom: 15 }
    );
  }, [pins]);

  useEffect(() => {
    userMoved.current = false;
    const t = window.setTimeout(() => fitPins(true), 120);
    return () => window.clearTimeout(t);
  }, [pins, fitPins]);

  useEffect(() => {
    if (!selectedId) return;
    const pin = pins.find((p) => p.id === selectedId);
    if (!pin || !mapRef.current) return;
    mapRef.current.flyTo({
      center: [pin.longitude, pin.latitude],
      zoom: Math.max(mapRef.current.getZoom(), 13),
      duration: 650,
    });
  }, [selectedId, pins]);

  return (
    <div className="map-canvas">
      <Map
        ref={mapRef}
        initialViewState={initialView}
        mapStyle={style}
        style={{ width: "100%", height: "100%" }}
        onDragStart={() => { userMoved.current = true; }}
        onZoomStart={() => { userMoved.current = true; }}
      >
        <NavigationControl position="bottom-right" showCompass={false} />
        {pins.map((pin, i) => (
          <Marker
            key={pin.id}
            longitude={pin.longitude}
            latitude={pin.latitude}
            anchor="center"
            onClick={(e) => {
              e.originalEvent.stopPropagation();
              onSelect(pin);
            }}
          >
            <div
              className={markerClass(pin, pin.id === selectedId)}
              style={{ animationDelay: `${Math.min(i, 12) * 40}ms` }}
              title={`${pin.name} (${pin.kind})`}
              role="button"
              aria-label={pin.name}
            />
          </Marker>
        ))}
      </Map>
    </div>
  );
}
