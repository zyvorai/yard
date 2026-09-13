import type {ReactNode} from 'react';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import useBaseUrl from '@docusaurus/useBaseUrl';
import styles from './gallery.module.css';

type Shot = {
  src: string;
  caption: string;
};

const TOUR: Shot[] = [
  {src: '/00-overview.png', caption: 'Overview'},
  {src: '/01-assets.png', caption: 'Assets (search, kinds, bulk import/export)'},
  {src: '/02-sites.png', caption: 'Sites'},
  {src: '/03-map.png', caption: 'Map (clustered pins)'},
  {src: '/04-telemetry.png', caption: 'Telemetry'},
  {src: '/05-work-orders.png', caption: 'Work orders'},
  {src: '/06-automations.png', caption: 'Automations'},
];

function ShotCard({shot}: {shot: Shot}) {
  const src = useBaseUrl(shot.src);
  return (
    <figure className={styles.shot}>
      <img src={src} alt={shot.caption} loading="lazy" />
      <figcaption>{shot.caption}</figcaption>
    </figure>
  );
}

export default function Gallery(): ReactNode {
  const hero = useBaseUrl('/00-overview.png');
  return (
    <Layout
      title="Gallery"
      description="A walkthrough of the Yard console, captured against a live lab deployment.">
      <header className={styles.header}>
        <div className="container">
          <Heading as="h1">Product tour</Heading>
          <p>
            Every screenshot below is captured against a real, running lab
            deployment — not a mockup.
          </p>
        </div>
      </header>
      <main className="container">
        <div className={styles.demo}>
          <img
            src={hero}
            alt="Yard Overview — health counters, incidents, and activity"
          />
          <p className={styles.caption}>
            Live lab: Overview health and incidents, clustered MapLibre map,
            telemetry with freshness, automations, and severity runbooks.
          </p>
        </div>
        <div className={styles.grid}>
          {TOUR.map((shot) => (
            <ShotCard key={shot.src} shot={shot} />
          ))}
        </div>
      </main>
    </Layout>
  );
}
