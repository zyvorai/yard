import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type Shot = {
  src: string;
  alt: string;
};

const SHOTS: Shot[] = [
  {src: '/00-overview.png', alt: 'Yard Overview — health, incidents, activity'},
  {src: '/03-map.png', alt: 'Yard Map — MapLibre full-bleed with clustered pins'},
  {src: '/04-telemetry.png', alt: 'Yard Telemetry — fresh measurements and sparklines'},
];

export default function ScreenshotStrip(): ReactNode {
  return (
    <section className={styles.strip}>
      <div className="container">
        <Heading as="h2" className="text--center">
          A real product, not a mockup
        </Heading>
        <p className="text--center">
          Captured against a live lab deployment.{' '}
          <Link to="/gallery">See the full tour →</Link>
        </p>
        <div className={styles.grid}>
          {SHOTS.map((shot) => (
            <ShotImage key={shot.src} shot={shot} />
          ))}
        </div>
      </div>
    </section>
  );
}

function ShotImage({shot}: {shot: Shot}) {
  const src = useBaseUrl(shot.src);
  return (
    <Link to="/gallery" className={styles.frame}>
      <img src={src} alt={shot.alt} loading="lazy" />
    </Link>
  );
}
