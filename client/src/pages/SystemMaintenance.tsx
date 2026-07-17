import IconColor from '../assets/brand/Icon-color.svg';

export default function SystemMaintenance() {
  return <main className="grid min-h-screen place-items-center bg-background p-6 text-on-surface">
    <section className="max-w-md text-center">
      <img src={IconColor} className="mx-auto h-16 w-16" alt="Rayavriti NetMonitor" />
      <p className="mt-8 text-xs font-label uppercase tracking-[0.2em] text-primary">Rayavriti NetMonitor</p>
      <h1 className="mt-3 font-headline text-3xl font-semibold">System Under Maintenance</h1>
      <p className="mt-4 text-sm leading-6 text-on-surface-variant">This NetMonitor instance is temporarily unavailable for operational updates. Please contact your administrator for assistance.</p>
    </section>
  </main>;
}
