import { useCallback, useEffect, useMemo, useRef } from "react";
import Map, { Layer, NavigationControl, Source, type MapLayerMouseEvent, type MapRef } from "react-map-gl/maplibre";
import type { GeoJSONSource } from "maplibre-gl";
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

export default function YardMap({ pins, selectedId, onSelect }: Props) {
  const { theme } = useTheme();
  const mapRef = useRef<MapRef>(null);
  const userMoved = useRef(false);
  const style = theme === "dark" ? MAP_STYLE_DARK : MAP_STYLE_LIGHT;
  const accentHex = theme === "dark" ? "#0a84ff" : "#0071e3";

  const geojson = useMemo(() => ({
    type: "FeatureCollection" as const,
    features: pins.map((p) => ({
      type: "Feature" as const,
      properties: {
        id: p.id,
        name: p.name,
        kind: p.kind,
        entity: p.entity,
        tone: p.entity === "site" ? "site" : (p.health || "unknown"),
      },
      geometry: {
        type: "Point" as const,
        coordinates: [p.longitude, p.latitude] as [number, number],
      },
    })),
  }), [pins]);

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

  const pinIdsKey = useMemo(() => pins.map((p) => p.id).sort().join(","), [pins]);
  const prevPinIdsKey = useRef<string | null>(null);

  useEffect(() => {
    const changed = prevPinIdsKey.current !== pinIdsKey;
    prevPinIdsKey.current = pinIdsKey;
    if (!changed || userMoved.current) return;
    const t = window.setTimeout(() => fitPins(true), 120);
    return () => window.clearTimeout(t);
  }, [pinIdsKey, fitPins]);

  const recenter = useCallback(() => {
    userMoved.current = false;
    fitPins(true);
  }, [fitPins]);

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

  const onClick = useCallback((e: MapLayerMouseEvent) => {
    const feat = e.features?.[0];
    if (!feat || feat.geometry.type !== "Point") return;
    const map = mapRef.current?.getMap();
    if (!map) return;
    const coords = feat.geometry.coordinates as [number, number];
    if (feat.properties?.cluster) {
      const source = map.getSource("pins") as GeoJSONSource | undefined;
      const clusterId = feat.properties.cluster_id as number;
      if (!source || clusterId == null) return;
      void source.getClusterExpansionZoom(clusterId).then((zoom) => {
        map.easeTo({ center: coords, zoom, duration: 450 });
      });
      return;
    }
    const id = String(feat.properties?.id || "");
    const pin = pins.find((p) => p.id === id);
    if (pin) onSelect(pin);
  }, [onSelect, pins]);

  return (
    <div className="map-canvas">
      <button type="button" className="map-recenter" onClick={recenter}>Recenter</button>
      <Map
        ref={mapRef}
        initialViewState={initialView}
        mapStyle={style}
        style={{ width: "100%", height: "100%" }}
        interactiveLayerIds={["clusters", "unclustered-point"]}
        onClick={onClick}
        onDragStart={() => { userMoved.current = true; }}
        onZoomStart={() => { userMoved.current = true; }}
        cursor="pointer"
      >
        <NavigationControl position="bottom-right" showCompass={false} />
        <Source
          id="pins"
          type="geojson"
          data={geojson}
          cluster
          clusterMaxZoom={14}
          clusterRadius={56}
        >
          <Layer
            id="clusters"
            type="circle"
            filter={["has", "point_count"]}
            paint={{
              "circle-color": ["step", ["get", "point_count"], "#0a84ff", 8, "#ff9f0a", 25, "#004a99"],
              "circle-radius": ["step", ["get", "point_count"], 18, 8, 24, 25, 32],
              "circle-stroke-width": 2,
              "circle-stroke-color": "#ffffff",
            }}
          />
          <Layer
            id="cluster-count"
            type="symbol"
            filter={["has", "point_count"]}
            layout={{
              "text-field": ["get", "point_count_abbreviated"],
              "text-size": 12,
            }}
            paint={{ "text-color": "#ffffff" }}
          />
          <Layer
            id="unclustered-point"
            type="circle"
            filter={["!", ["has", "point_count"]]}
            paint={{
              "circle-color": [
                "match",
                ["get", "tone"],
                "healthy", "#34c759",
                "degraded", "#ff9f0a",
                "warning", "#ff9f0a",
                "critical", "#ff3b30",
                "stale", "#8e8e97",
                "site", "#0a84ff",
                "#8e8e97",
              ],
              "circle-radius": ["case", ["==", ["get", "id"], selectedId || ""], 10, 7],
              "circle-stroke-width": ["case", ["==", ["get", "id"], selectedId || ""], 3, 2],
              "circle-stroke-color": ["case", ["==", ["get", "id"], selectedId || ""], accentHex, "#ffffff"],
            }}
          />
        </Source>
      </Map>
    </div>
  );
}
