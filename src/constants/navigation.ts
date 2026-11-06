/**
 * Navigation is data, not markup. The sidebar and the router both read this
 * list, so a route can never exist without a menu entry (or the reverse).
 */

export interface NavItem {
  path: string;
  label: string;
  /** Stage that will implement it. Pure documentation for the user. */
  stage: number;
  /** Implemented already — no "S<n>" badge in the sidebar. */
  built?: boolean;
}

export interface NavGroup {
  title: string;
  items: NavItem[];
}

export const NAV_GROUPS: NavGroup[] = [
  {
    title: "Monitoring",
    items: [
      { path: "/", label: "Overview", stage: 1, built: true },
      { path: "/olts", label: "OLT Inventory", stage: 3, built: true },
      { path: "/pons", label: "PON Explorer", stage: 3, built: true },
      { path: "/onus", label: "ONU Management", stage: 4, built: true },
      { path: "/incidents", label: "Incidents & Events", stage: 5, built: true },
    ],
  },
  {
    title: "Operations",
    items: [
      { path: "/optical", label: "Optical Diagnostics", stage: 4, built: true },
      { path: "/reports", label: "History & Reports", stage: 6 },
      { path: "/collection", label: "Collection Health", stage: 2, built: true },
    ],
  },
  {
    title: "System",
    items: [
      { path: "/console", label: "Diagnostics Console", stage: 7 },
      { path: "/users", label: "Users & Access", stage: 7 },
      { path: "/settings", label: "Settings", stage: 7 },
    ],
  },
];

export const ALL_NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((group) => group.items);
