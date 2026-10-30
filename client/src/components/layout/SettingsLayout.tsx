import { Outlet } from 'react-router-dom';
import SectionNav from '../ui/SectionNav';
import { NAV_GROUPS, visibleSections } from '../../navigation';
import { useSessionUser } from '../../store/session';

const SETTINGS = NAV_GROUPS.flatMap((g) => g.items).find((i) => i.id === 'settings');

/** /settings/*: side tabs (a scrolling strip on narrow screens) beside the section. */
export default function SettingsLayout() {
  const user = useSessionUser();
  const sections = visibleSections(SETTINGS?.sections, user);
  return (
    <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
      <aside className="lg:sticky lg:top-20 lg:w-56 lg:shrink-0">
        <h2 className="mb-2 hidden px-3 text-xs font-semibold text-fg-muted lg:block">Settings</h2>
        <SectionNav sections={sections} label="Settings sections" variant="side" />
      </aside>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  );
}
