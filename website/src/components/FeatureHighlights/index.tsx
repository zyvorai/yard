import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  description: ReactNode;
  to: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Asset registry',
    description:
      'Devices, vehicles, machines, sensors, and equipment — with capabilities, sites, and health that stays honest when data goes stale.',
    to: '/docs/core-concepts/architecture',
  },
  {
    title: 'Live ops loop',
    description:
      'Background stale ticker, SSE-driven Overview and Map, and browser notifications for critical incidents — no manual refresh.',
    to: '/docs/getting-started/quickstart',
  },
  {
    title: 'Sites & map',
    description:
      'Factories, warehouses, and customer locations on a full-bleed MapLibre map with dark basemap and glass detail panel.',
    to: '/docs/core-concepts/architecture',
  },
  {
    title: 'Incidents & work',
    description:
      'Threshold and stale automations open incidents. Standalone work orders close the loop on the asset timeline.',
    to: '/docs/getting-started/quickstart',
  },
  {
    title: 'Automations & webhooks',
    description:
      'Create threshold or stale rules that open incidents, notify, or POST to your webhook URL.',
    to: '/docs/guides/connectors',
  },
  {
    title: 'Optional connectors',
    description:
      'HTTP ingest and Device Agent today. Nodra, Fleet, and OTA stay catalog-ready without owning the core model.',
    to: '/docs/guides/connectors',
  },
];

function Feature({title, description, to}: FeatureItem) {
  return (
    <div className="col col--4">
      <Link to={to} className={styles.card}>
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </Link>
    </div>
  );
}

export default function FeatureHighlights(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
