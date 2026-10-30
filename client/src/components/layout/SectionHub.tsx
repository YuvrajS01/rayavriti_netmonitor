import { Navigate, Outlet, useLocation, useParams } from 'react-router-dom';
import SectionNav from '../ui/SectionNav';
import { DISCOVERY_SECTIONS, NAV_GROUPS, visibleSections, type NavSection } from '../../navigation';
import { useSessionUser } from '../../store/session';

function HubTabs({ sections, label }: { sections: NavSection[]; label: string }) {
  const user = useSessionUser();
  const visible = visibleSections(sections, user);
  return (
    <>
      {visible.length > 1 && <SectionNav sections={visible} label={label} />}
      <Outlet />
    </>
  );
}

/** A nav item with section tabs (Alerts & incidents, Topology, …). */
export function SectionHub({ itemId }: { itemId: string }) {
  const item = NAV_GROUPS.flatMap((g) => g.items).find((i) => i.id === itemId);
  if (!item?.sections) return <Outlet />;
  return <HubTabs sections={item.sections} label={`${item.label} sections`} />;
}

/** Settings › Discovery & import: Discovery | Bulk import. */
export function DiscoveryHub() {
  return <HubTabs sections={DISCOVERY_SECTIONS} label="Discovery and import sections" />;
}

/**
 * Redirect an old URL to its new home, carrying `:id` and keeping the
 * query string and hash (merged with any query the target already has).
 */
export function LegacyRedirect({ to }: { to: string }) {
  const params = useParams();
  const location = useLocation();
  const [rawPath, rawQuery = ''] = to.split('?');
  const path = rawPath!.replace(/:([a-zA-Z]+)/g, (_, key: string) => encodeURIComponent(params[key] ?? ''));
  const query = new URLSearchParams(rawQuery);
  new URLSearchParams(location.search).forEach((value, key) => {
    if (!query.has(key)) query.set(key, value);
  });
  // /campus?view=floor was the floor-plan view; it has its own tab now.
  if (path === '/topology/campus' && query.get('view') === 'floor') {
    query.delete('view');
    return <Navigate replace to={{ pathname: '/topology/floor-plans', search: query.toString() ? `?${query}` : '', hash: location.hash }} />;
  }
  // The generated-reports tab of the old builder is the Library now.
  if (path === '/reports/scheduled' && query.get('tab') === 'generated') {
    query.delete('tab');
    return <Navigate replace to={{ pathname: '/reports/library', search: query.toString() ? `?${query}` : '', hash: location.hash }} />;
  }
  const search = query.toString();
  return <Navigate replace to={{ pathname: path, search: search ? `?${search}` : '', hash: location.hash }} />;
}
