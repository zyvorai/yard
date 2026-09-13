export type FeatureEntry = {
  title: string;
  description: string;
  to: string;
  image: string;
  caption: string;
};

/**
 * Shared feature copy + screenshot mapping, consumed by both the homepage's
 * scroll story (FeatureScrollStory) and the /tour deep-dive page, so the
 * two never drift out of sync.
 */
export const FEATURES: FeatureEntry[] = [
  {
    title: 'Asset registry',
    description:
      'Devices, vehicles, machines, sensors, and equipment — with capabilities, sites, and health that stays honest when data goes stale.',
    to: '/docs/core-concepts/architecture',
    image: '/01-assets.png',
    caption:
      'Every asset carries capabilities, a site, and a health state that decays honestly when data goes stale.',
  },
  {
    title: 'Live ops loop',
    description:
      'Background stale ticker, SSE-driven Overview and Map, and browser notifications for critical incidents — no manual refresh.',
    to: '/docs/getting-started/quickstart',
    image: '/00-overview.png',
    caption: 'Health, open incidents, and recent activity update live over SSE — no refresh button.',
  },
  {
    title: 'Sites & map',
    description:
      'Factories, warehouses, and customer locations on a full-bleed MapLibre map with dark basemap and glass detail panel.',
    to: '/docs/core-concepts/architecture',
    image: '/03-map.png',
    caption: 'A full-bleed MapLibre map, Find My style — glass detail panel, live marker health.',
  },
  {
    title: 'Incidents & work',
    description:
      'Threshold and stale automations open incidents. Standalone work orders close the loop on the asset timeline.',
    to: '/docs/getting-started/quickstart',
    image: '/05-work-orders.png',
    caption: 'Work orders close the loop — every resolution lands back on the asset timeline.',
  },
  {
    title: 'Automations & webhooks',
    description: 'Create threshold or stale rules that open incidents, notify, or POST to your webhook URL.',
    to: '/docs/guides/connectors',
    image: '/06-automations.png',
    caption: 'Threshold and stale rules that open incidents, notify, or POST to your own webhook.',
  },
  {
    title: 'Optional connectors',
    description:
      'HTTP ingest and Device Agent today. Nodra, Fleet, and OTA stay catalog-ready without owning the core model.',
    to: '/docs/guides/connectors',
    image: '/04-telemetry.png',
    caption: 'Telemetry flows in from HTTP ingest or Device Agent — Nodra, Fleet, and OTA stay optional.',
  },
];
